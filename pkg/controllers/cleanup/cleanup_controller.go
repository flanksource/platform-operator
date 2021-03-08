/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cleanup

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	name = "cleanup-controller"

	// this is the label used for the lookup of the ns to clean-up (i.e auto-delete=24h)
	cleanupLabel = "auto-delete"
)

var log = logf.Log.WithName(name)

func Add(mgr manager.Manager, interval time.Duration) error {
	return add(mgr, newReconciler(mgr, interval))
}

func newReconciler(mgr manager.Manager, interval time.Duration) reconcile.Reconciler {
	return &ReconcileCleanup{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		interval: interval,
	}
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForObject{},
		predicate.Funcs{CreateFunc: onCreate, UpdateFunc: onUpdate},
	)
}

var _ reconcile.Reconciler = &ReconcileCleanup{}

type ReconcileCleanup struct {
	client.Client
	Scheme *runtime.Scheme

	// interval is the time after which the controller requeue the reconcile key
	interval time.Duration
}

// parseDuration parses strings into time.Duration with added support for day 'd' units
func parseDuration(expiry string) (*time.Duration, error) {
	if strings.HasSuffix(expiry, "d") {
		days, err := strconv.Atoi(expiry[0 : len(expiry)-1])
		if err != nil {
			return nil, err
		} else {
			expiry = fmt.Sprintf("%dh", days*24)
		}
	}
	duration, err := time.ParseDuration(expiry)
	return &duration, err
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;delete

func (r *ReconcileCleanup) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	namespace := corev1.Namespace{}
	if err := r.Get(ctx, request.NamespacedName, &namespace); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	if namespace.Status.Phase == corev1.NamespaceTerminating {
		log.V(1).Info("Namespace is already terminating", "namespace", namespace.Name)
		return reconcile.Result{}, nil
	}

	expiry := namespace.Labels[cleanupLabel]
	if expiry == "" {
		return reconcile.Result{}, nil
	}
	duration, err := parseDuration(expiry)
	if err != nil {
		log.Error(err, "Invalid duration for namespace", "namespace", namespace.Name, "expiry", expiry)
		return reconcile.Result{}, err
	}

	expiresOn := namespace.GetCreationTimestamp().Add(*duration)
	if expiresOn.Before(time.Now()) {
		log.V(1).Info("Deleting namespace", "namespace", namespace.Name, "expiry", expiry)

		if err := r.Delete(ctx, &namespace); err != nil {
			log.Error(err, "Failed to delete namespace", "namespace", namespace.Name)
			return reconcile.Result{Requeue: true}, err
		}
	}

	log.V(2).Info("Requeue reconciliation", "interval", r.interval)
	return reconcile.Result{RequeueAfter: r.interval}, nil
}

// these functions are passed to the predicate
// objects are queued only in case the label exists
func onCreate(e event.CreateEvent) bool {
	namespace := e.Object.(*corev1.Namespace)
	labels := namespace.GetLabels()
	_, isSet := labels[cleanupLabel]
	return isSet
}

func onUpdate(e event.UpdateEvent) bool {
	namespace := e.ObjectNew.(*corev1.Namespace)
	labels := namespace.GetLabels()
	_, isSet := labels[cleanupLabel]
	return isSet
}
