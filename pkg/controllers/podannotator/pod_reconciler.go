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

package podannotator

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// list of whitelisted annotations
	annotations []string
}

func newPodReconciler(mgr manager.Manager, annotations []string) reconcile.Reconciler {
	return &PodReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		annotations: annotations,
	}
}

func addPodReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	if err := c.Watch(
		&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{},
		predicate.Funcs{CreateFunc: onCreate, UpdateFunc: onUpdate},
	); err != nil {
		return err
	}

	return nil
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;update;watch

func (r *PodReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	pod := corev1.Pod{}
	if err := r.Get(ctx, request.NamespacedName, &pod); err != nil {
		return reconcile.Result{}, err
	}

	namespace := corev1.Namespace{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: request.Namespace}, &namespace); err != nil {
		return reconcile.Result{}, err
	}

	podsChanged := updatePodAnnotations(namespace, r.annotations, pod)

	for _, pod := range podsChanged {
		if err := r.Client.Update(ctx, &pod); err != nil {
			log.Error(err, "failed to update", "pod", request.Name, "namespace", request.Namespace)
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}
