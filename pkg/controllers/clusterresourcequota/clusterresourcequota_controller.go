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

package clusterresourcequota

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"time"

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	name = "clusterresourcequota-controller"
)

var log = logf.Log.WithName(name)

func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileClusterResourceQuota{
		mtx:    &sync.Mutex{},
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	if err := c.Watch(&source.Kind{Type: &platformv1.ClusterResourceQuota{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	fn := handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {

		// this function map a ResourceQuota event to a reconcile request for all ClusterResourceQuotas
		client := mgr.GetClient()

		quotaList := &platformv1.ClusterResourceQuotaList{}
		if err := client.List(context.Background(), quotaList); err != nil {
			return nil
		}

		if len(quotaList.Items) == 0 {
			return nil
		}

		var requests []reconcile.Request
		for _, quota := range quotaList.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: quota.GetNamespace(),
					Name:      quota.GetName(),
				},
			})
		}

		return requests
	})

	return c.Watch(&source.Kind{Type: &corev1.ResourceQuota{}}, fn)
}

var _ reconcile.Reconciler = &ReconcileClusterResourceQuota{}

type ReconcileClusterResourceQuota struct {
	mtx *sync.Mutex
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=platform.flanksource.com,resources=clusterresourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.flanksource.com,resources=clusterresourcequotas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch

func (r *ReconcileClusterResourceQuota) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	quota := &platformv1.ClusterResourceQuota{}
	if err := r.Get(ctx, request.NamespacedName, quota); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	existing, err := findMatchingResourceQuotas(ctx, r.Client, quota, nil)
	if err != nil {
		return reconcile.Result{}, err
	}

	quota.Status.Total.Hard = sumOfHard(existing)
	quota.Status.Total.Used = sumOfUsed(existing)
	quota.Status.Namespaces = platformv1.ResourceQuotasStatusByNamespace{}
	sum, keys := sumByNamespace(existing)

	for _, namespace := range keys {
		quota.Status.Namespaces = append(quota.Status.Namespaces, platformv1.ResourceQuotaStatusByNamespace{
			Namespace: namespace,
			Status:    sum[namespace].Status,
		})
	}

	if err := r.Client.Status().Update(ctx, quota); err != nil {
		if strings.Contains(err.Error(), "the object has been modified; please apply your changes to the latest version and try again") {
			log.Info("Concurrent update detected, retrying")
			return reconcile.Result{RequeueAfter: time.Second * time.Duration(1+rand.Intn(4))}, nil
		}
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
