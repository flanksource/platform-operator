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
	"time"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("cleanup-controller")

func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileCleanup{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New("cleanup-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

type ReconcileCleanup struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete

func (r *ReconcileCleanup) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	namespaces := corev1.NamespaceList{}

	expireLabel, err := labels.NewRequirement("auto-delete", selection.Exists, []string{})
	if err != nil {
		log.Error(err, "Failed to build new requirement")
		return reconcile.Result{}, err
	}

	expireLabelSelector := labels.NewSelector().Add(*expireLabel)
	expireLabelListOption := client.MatchingLabelsSelector{Selector: expireLabelSelector}

	err = r.List(ctx, &namespaces, expireLabelListOption)
	if err != nil {
		log.Error(err, "Failed to list namespaces")
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	for _, ns := range namespaces.Items {
		if ns.Status.Phase == corev1.NamespaceTerminating {
			log.V(1).Info("namespace is already terminating", "namespace", ns.Name)
			continue
		}

		expiry := ns.Labels["auto-delete"]

		duration, err := time.ParseDuration(expiry)
		if err != nil {
			log.Error(err, "Invalid duration for namespace", "namespace", ns.Name, "expiry", expiry)
			continue
		}

		expiresOn := ns.GetCreationTimestamp().Add(duration)
		if expiresOn.Before(time.Now()) {
			log.V(1).Info("Deleting namespace", "namespace", ns.Name)

			err = r.Delete(ctx, &ns)
			if err != nil {
				log.Error(err, "Failed to delete namespace", "namespace", ns.Name)
				continue
			}
		}
	}

	return reconcile.Result{}, nil
}
