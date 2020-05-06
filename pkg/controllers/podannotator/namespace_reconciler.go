package podannotator

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// interval is the time after which the controller requeue the reconcile key
	interval time.Duration
	// list of whitelisted annotations
	annotations []string
}

func newNamespaceReconciler(mgr manager.Manager, interval time.Duration, annotations []string) reconcile.Reconciler {
	return &NamespaceReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		interval:    interval,
		annotations: annotations,
	}
}

func addNamespaceReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	if err := c.Watch(
		&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForObject{},
		predicate.Funcs{CreateFunc: onCreate, UpdateFunc: onUpdate},
	); err != nil {
		return err
	}

	return nil
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;update

func (r *NamespaceReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	ns := corev1.Namespace{}
	if err := r.Get(ctx, request.NamespacedName, &ns); err != nil {
		return reconcile.Result{}, err
	}

	podList := corev1.PodList{}
	if err := r.Client.List(ctx, &podList, client.InNamespace(request.Name)); err != nil {
		return reconcile.Result{}, err
	}

	changedPods := updatePodAnnotations(ns, r.annotations, podList.Items...)

	for _, pod := range changedPods {
		if err := r.Client.Update(ctx, &pod); err != nil {
			log.Error(err, "failed to update pod %s in namespace %s", request.Name, request.Namespace)
			return reconcile.Result{}, err
		}
	}

	log.V(1).Info("Requeue reconciliation", "interval", r.interval)
	return reconcile.Result{RequeueAfter: r.interval}, nil
}
