package podannotator

import v1 "k8s.io/api/core/v1"

func updatePodAnnotations(ns v1.Namespace, annotations []string, pods ...v1.Pod) []v1.Pod {
	annotationsMap := map[string]bool{}
	for _, a := range annotations {
		annotationsMap[a] = true
	}

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
			if _, f := annotationsMap[k]; f { // if annotation is whitelisted
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
