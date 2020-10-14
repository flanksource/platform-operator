package podannotator

import (
	"time"

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	"github.com/pkg/errors"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	name = "pod-controller"
)

var log = logf.Log.WithName(name)

func Add(mgr manager.Manager, interval time.Duration, cfg platformv1.PodMutaterConfig) error {
	if err := addPodReconciler(mgr, newPodReconciler(mgr, cfg)); err != nil {
		return errors.Wrap(err, "failed to add pod reconciler")
	}

	if err := addNamespaceReconciler(mgr, newNamespaceReconciler(mgr, interval, cfg)); err != nil {
		return errors.Wrap(err, "failed to add namespace reconciler")
	}

	return nil
}
