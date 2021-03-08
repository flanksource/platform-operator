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

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilquota "k8s.io/apiserver/pkg/quota/v1"
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
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	if err := c.Watch(
		&source.Kind{Type: &platformv1.ClusterResourceQuota{}}, &handler.EnqueueRequestForObject{},
	); err != nil {
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
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=platform.flanksource.com,resources=clusterresourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.flanksource.com,resources=clusterresourcequotas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch
func (r *ReconcileClusterResourceQuota) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	quota := &platformv1.ClusterResourceQuota{}
	if err := r.Get(ctx, request.NamespacedName, quota); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		log.Error(err, "Failed to get Cluster Resource Quota")
		return reconcile.Result{}, err
	}

	namespacesList := &corev1.NamespaceList{}
	if err := r.List(ctx, namespacesList); err != nil {
		log.Error(err, "Failed to list namespaces")
		return reconcile.Result{}, err
	}

	for _, namespace := range namespacesList.Items {
		namespaceName := namespace.Name

		// get the quotas per namespace in the status of the ClusterQuotaResource
		// this represent the past - here below we need to compute the future
		namespaceTotals := GetResourceQuotasStatusByNamespace(quota.Status.Namespaces, namespaceName)

		rqList := &corev1.ResourceQuotaList{}
		if err := r.List(ctx, rqList, client.InNamespace(namespaceName)); err != nil {
			log.Error(err, "Failed to list Resource Quota", "namespace", namespaceName)
			return reconcile.Result{}, err
		}

		if len(rqList.Items) == 0 {
			log.Info("Warning: no ResourceQuota defined", "namespace", namespaceName)
			continue
		}

		// calculate the quotas on a single namespace
		var recalculatedStatus corev1.ResourceQuotaStatus = corev1.ResourceQuotaStatus{}
		for _, rq := range rqList.Items {
			// calculate the status across all the Resource Quota in a namespace
			usedCurrent := utilquota.Add(recalculatedStatus.Used, rq.Status.Used)
			hardCurrent := utilquota.Add(recalculatedStatus.Hard, rq.Status.Hard)

			recalculatedStatus = corev1.ResourceQuotaStatus{
				Used: usedCurrent,
				Hard: hardCurrent,
			}
		}

		// subtract old usage, add new usage
		quota.Status.Total.Used = utilquota.Subtract(quota.Status.Total.Used, namespaceTotals.Used)
		quota.Status.Total.Used = utilquota.Add(quota.Status.Total.Used, recalculatedStatus.Used)
		InsertResourceQuotasStatus(&quota.Status.Namespaces, platformv1.ResourceQuotaStatusByNamespace{
			Namespace: namespaceName,
			Status:    recalculatedStatus,
		})
	}

	statusCopy := quota.Status.Namespaces.DeepCopy()
	for _, namespaceTotals := range statusCopy {
		namespaceName := namespaceTotals.Namespace

		rqList := &corev1.ResourceQuotaList{}
		if err := r.List(ctx, rqList, client.InNamespace(namespaceName)); err != nil {
			log.Error(err, "Failed to list Resource Quota", "namespace", namespaceName)
			return reconcile.Result{}, err
		}

		if len(rqList.Items) == 0 {
			quota.Status.Total.Used = utilquota.Subtract(quota.Status.Total.Used, namespaceTotals.Status.Used)
			RemoveResourceQuotasStatusByNamespace(&quota.Status.Namespaces, namespaceName)
		}
	}

	quota.Status.Total.Hard = quota.Spec.Quota.Hard
	if err := r.Client.Update(ctx, quota); err != nil {
		log.Error(err, "Failed to update status")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
