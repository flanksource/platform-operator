package pod

import (
	"context"
	"time"

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// interval is the time after which the controller requeue the reconcile key
	interval time.Duration
	cfg      platformv1.PodMutaterConfig
}

func newNamespaceReconciler(mgr manager.Manager, interval time.Duration, cfg platformv1.PodMutaterConfig) reconcile.Reconciler {
	return &NamespaceReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		interval: interval,
		cfg:      cfg,
	}
}

func addNamespaceReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	return c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForObject{})
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;update
func (r *NamespaceReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	ns := corev1.Namespace{}
	if err := r.Get(ctx, request.NamespacedName, &ns); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	podList := corev1.PodList{}
	if err := r.Client.List(ctx, &podList, client.InNamespace(request.Name)); err != nil {
		return reconcile.Result{}, err
	}

	changedPods := RequiresAnnotationUpdate(ns, r.cfg, podList.Items...)

	for _, pod := range changedPods {
		if err := r.Client.Update(ctx, &pod); err != nil {
			log.Error(err, "failed to update pod %s in namespace %s", request.Name, request.Namespace)
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{RequeueAfter: r.interval}, nil
}
