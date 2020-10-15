package podannotator

import (
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

		for k, v := range ns.Annotations {
			if _, f := cfg.AnnotationsMap[k]; f { // if annotation is whitelisted
				if _, podHasAnnotation := pod.Annotations[k]; !podHasAnnotation { // if pod already has annotation, don't inherit
					pod.Annotations[k] = v
					podChanged = true
				}
			}
		}

		if podChanged {
			changedPods = append(changedPods, pod)
		}
	}

	return changedPods
}
