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

		podChanged := false

		for k1, v1 := range ns.Annotations {
			for k2, v2 := range cfg.AnnotationsMap {
				if v2 && strings.HasPrefix(k1, k2) { // prefix matching
					if _, podHasAnnotation := pod.Annotations[k1]; !podHasAnnotation { // if pod already has annotation, don't inherit
						pod.Annotations[k1] = v1
						podChanged = true
					}
				}
			}
		}

		if podChanged {
			changedPods = append(changedPods, pod)
		}
	}

	return changedPods
}
