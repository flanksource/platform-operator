package podannotator

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	name = "pod-annotator-controller"
)

func Add(mgr manager.Manager, interval time.Duration, annotations []string) error {
	return add(mgr, newReconciler(mgr, interval, annotations))
}

func newReconciler(mgr manager.Manager, interval time.Duration, annotations []string) reconcile.Reconciler {
	annotationsMap := map[string]bool{}
	for _, a := range annotations {
		annotationsMap[a] = true
	}

	return &ReconcilePodAnnotations{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		interval:    interval,
		annotations: annotationsMap,
		log:         logf.Log.WithName(name),
	}
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
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

var _ reconcile.Reconciler = &ReconcilePodAnnotations{}

type ReconcilePodAnnotations struct {
	client.Client
	Scheme *runtime.Scheme

	// interval is the time after which the controller requeue the reconcile key
	interval time.Duration
	// list of whitelisted annotations
	annotations map[string]bool

	log logr.Logger
}

// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;update
func (r *ReconcilePodAnnotations) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	pod := corev1.Pod{}
	if err := r.Get(ctx, request.NamespacedName, &pod); err != nil {
		return reconcile.Result{}, err
	}

	namespace := corev1.Namespace{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: request.Namespace}, &namespace); err != nil {
		return reconcile.Result{}, err
	}

	if namespace.Annotations == nil {
		namespace.Annotations = map[string]string{}
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}

	for k, v := range namespace.Annotations {
		if _, f := r.annotations[k]; f { // if annotation is whitelisted
			if _, podHasAnnotation := pod.Annotations[k]; !podHasAnnotation { // if pod already has annotation, don't inherit
				pod.Annotations[k] = v
			}
		}
	}

	if err := r.Client.Update(ctx, &pod); err != nil {
		r.log.Error(err, "failed to update pod %s in namespace %s", request.Name, request.Namespace)
		return reconcile.Result{}, err
	}

	r.log.V(1).Info("Requeue reconciliation", "interval", r.interval)
	return reconcile.Result{RequeueAfter: r.interval}, nil
}

// these functions are passed to the predicate
// objects are queued only in case the label exists
func onCreate(e event.CreateEvent) bool {
	return true
}

func onUpdate(e event.UpdateEvent) bool {
	return true
}
