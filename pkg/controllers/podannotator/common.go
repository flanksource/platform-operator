package podannotator

import (
	"strings"

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	v1 "k8s.io/api/core/v1"
)

func updatePods(ns v1.Namespace, cfg platformv1.PodMutaterConfig, pods ...v1.Pod) []v1.Pod {
	changedPods := []v1.Pod{}

	if ns.Annotations == nil {
		ns.Annotations = map[string]string{}
	}

	for _, pod := range pods {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}

		podChanged := checkUpdatePod(ns, cfg, pod)

		if podChanged {
			changedPods = append(changedPods, pod)
		}
	}

	return changedPods
}

func checkUpdatePod(ns v1.Namespace, cfg platformv1.PodMutaterConfig, pod v1.Pod) bool {
	podChanged := false

	for nsAnnotationKey, nsAnnotationVal := range ns.Annotations {
		for whitelistAnnotation, whitelistBool := range cfg.AnnotationsMap {
			if whitelistBool && strings.HasPrefix(nsAnnotationKey, whitelistAnnotation) { // prefix matching
				if _, podHasAnnotation := pod.Annotations[nsAnnotationKey]; !podHasAnnotation { // if pod already has annotation, don't inherit
					pod.Annotations[nsAnnotationKey] = nsAnnotationVal
					podChanged = true
				}
			}
		}
	}
	return podChanged
}
