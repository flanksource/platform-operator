package ingress

import (
	"time"

	"github.com/pkg/errors"

	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	name = "ingress-annotator-controller"
)

var log = logf.Log.WithName(name)

func Add(mgr manager.Manager, interval time.Duration, svcName, svcNamespace, domain string) error {
	if err := addIngressReconciler(mgr, newIngressReconciler(mgr, svcName, svcNamespace, domain)); err != nil {
		return errors.Wrap(err, "failed to add ingress reconciler")
	}

	return nil
}

var _ reconcile.Reconciler = &IngressReconciler{}

// these functions are passed to the predicate
// objects are queued only in case the label exists
func onCreate(e event.CreateEvent) bool {
	return true
}

func onUpdate(e event.UpdateEvent) bool {
	return true
}
