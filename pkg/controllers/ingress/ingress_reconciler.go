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

package ingress

import (
	"context"

	utilsk8s "github.com/flanksource/platform-operator/pkg/controllers/utils/k8s"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type IngressReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ingressAnnotator *utilsk8s.IngressAnnotator
}

func newIngressReconciler(mgr manager.Manager, svcName, svcNamespace, domain string) reconcile.Reconciler {
	client := mgr.GetClient()
	ingressAnnotator := utilsk8s.NewIngressAnnotator(client, svcName, svcNamespace, domain)

	return &IngressReconciler{
		Client:           client,
		Scheme:           mgr.GetScheme(),
		ingressAnnotator: ingressAnnotator,
	}
}

func addIngressReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	if err := c.Watch(
		&source.Kind{Type: &v1beta1.Ingress{}}, &handler.EnqueueRequestForObject{},
		predicate.Funcs{CreateFunc: onCreate, UpdateFunc: onUpdate},
	); err != nil {
		return err
	}

	return nil
}

// +kubebuilder:rbac:groups="extensions",resources=ingresses,verbs=get;list;update;watch

func (r *IngressReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	ingress := &v1beta1.Ingress{}
	if err := r.Get(ctx, request.NamespacedName, ingress); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	updatedIngress, changed, err := r.ingressAnnotator.Annotate(ctx, ingress)
	if err != nil || !changed {
		return reconcile.Result{}, err
	}

	if err := r.Client.Update(ctx, updatedIngress); err != nil {
		log.Error(err, "failed to update", "ingress", updatedIngress.Name, "namespace", updatedIngress.Namespace)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
