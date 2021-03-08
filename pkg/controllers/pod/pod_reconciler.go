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

package pod

import (
	"context"
	"strings"

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
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
	Config platformv1.PodMutaterConfig
}

func NewPodReconciler(mgr manager.Manager, cfg platformv1.PodMutaterConfig) reconcile.Reconciler {
	cfg.AnnotationsMap = make(map[string]bool)
	for _, a := range cfg.Annotations {
		cfg.AnnotationsMap[a] = true
	}
	return &PodReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Config: cfg,
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
func (r *PodReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	pod := corev1.Pod{}
	if err := r.Get(ctx, request.NamespacedName, &pod); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{Requeue: true}, err
	}
	log.V(2).Info("Reconciling", "namespace", pod.Namespace, "pod", pod.Name)
	namespace := corev1.Namespace{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: request.Namespace}, &namespace); err != nil {

		log.Error(err, "Namespace not found", "namespace", pod.Namespace, "pod", pod.Name)
		return reconcile.Result{Requeue: true}, err
	}

	podsChanged := RequiresAnnotationUpdate(namespace, r.Config, pod)
	if len(podsChanged) == 0 {
		log.V(2).Info("Nothing to update", "namespace", pod.Namespace, "pod", pod.Name)
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

func RequiresAnnotationUpdate(ns v1.Namespace, cfg platformv1.PodMutaterConfig, pods ...v1.Pod) []v1.Pod {
	changedPods := []v1.Pod{}
	for _, pod := range pods {
		if updated, changed := UpdateAnnotations(ns, cfg, &pod); changed {
			changedPods = append(changedPods, *updated)
		}
	}
	return changedPods
}

func UpdateAnnotations(ns v1.Namespace, cfg platformv1.PodMutaterConfig, pod *v1.Pod) (*v1.Pod, bool) {
	if ns.Annotations == nil {
		ns.Annotations = map[string]string{}
	}

	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}

	changed := false
	for k, v := range ns.Annotations {
		if !isWhitelisted(k, cfg) {
			continue
		}

		if _, exists := pod.Annotations[k]; exists {
			// if pod already has annotation, don't inherit
			continue
		}
		pod.Annotations[k] = v
		changed = true
	}

	return pod, changed
}

func isWhitelisted(annotation string, cfg platformv1.PodMutaterConfig) bool {
	for key := range cfg.AnnotationsMap {
		if strings.HasPrefix(annotation, key) {
			return true
		}
	}
	return false
}
