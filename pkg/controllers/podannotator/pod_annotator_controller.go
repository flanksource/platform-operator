package podannotator

import (
	"time"

	"github.com/pkg/errors"

	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	name = "pod-annotator-controller"
)

var log = logf.Log.WithName(name)

func Add(mgr manager.Manager, interval time.Duration, annotations []string) error {
	if err := addPodReconciler(mgr, newPodReconciler(mgr, annotations)); err != nil {
		return errors.Wrap(err, "failed to add pod reconciler")
	}

	if err := addNamespaceReconciler(mgr, newNamespaceReconciler(mgr, interval, annotations)); err != nil {
		return errors.Wrap(err, "failed to add namespace reconciler")
	}

	return nil
}

var _ reconcile.Reconciler = &PodReconciler{}
var _ reconcile.Reconciler = &NamespaceReconciler{}

// these functions are passed to the predicate
// objects are queued only in case the label exists
func onCreate(e event.CreateEvent) bool {
	return true
}

func onUpdate(e event.UpdateEvent) bool {
	return true
}
