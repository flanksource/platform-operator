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

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	cfg    platformv1.PodMutaterConfig
}

func newPodReconciler(mgr manager.Manager, cfg platformv1.PodMutaterConfig) reconcile.Reconciler {
	cfg.AnnotationsMap = make(map[string]bool)
	for _, a := range cfg.Annotations {
		cfg.AnnotationsMap[a] = true
	}
	return &PodReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		cfg:    cfg,
	}
}

func addPodReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	return c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{})
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;update;watch
func (r *PodReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Info("Reconciling", "request", request)
	ctx := context.Background()
	pod := corev1.Pod{}
	if err := r.Get(ctx, request.NamespacedName, &pod); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{Requeue: true}, err
	}
	log.Info("Reconciling", "namespace", pod.Namespace, "pod", pod.Name)
	namespace := corev1.Namespace{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: request.Namespace}, &namespace); err != nil {

		log.Error(err, "Namespace not found", "namespace", pod.Namespace, "pod", pod.Name)
		return reconcile.Result{Requeue: true}, err
	}

	podsChanged := updatePods(namespace, r.cfg, pod)
	if len(podsChanged) == 0 {
		log.Info("Nothing to update", "namespace", pod.Namespace, "pod", pod.Name)
	}
	for _, pod := range podsChanged {
		if err := r.Client.Update(ctx, &pod); err != nil {
			log.Error(err, "failed to update", "namespace", pod.Namespace, "pod", pod.Name)
			return reconcile.Result{}, err
		} else {
			log.Info("Updated", "namespace", pod.Namespace, "pod", pod.Name)
		}
	}

	return reconcile.Result{}, nil
}
