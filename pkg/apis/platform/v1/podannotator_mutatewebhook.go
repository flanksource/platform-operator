// +kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod.kb.io
package v1

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=annotate-pods-v1.platform.flanksource.com
func PodAnnotatorMutateWebhook(client client.Client, annotations []string) *admission.Webhook {
	return &admission.Webhook{
		Handler: NewPodAnnotatorHandler(client, annotations),
	}
}

type podAnnotatorHandler struct {
	Client      client.Client
	decoder     *admission.Decoder
	annotations map[string]bool
}

func NewPodAnnotatorHandler(client client.Client, annotations []string) *podAnnotatorHandler {
	annotationsMap := map[string]bool{}

	for _, a := range annotations {
		annotationsMap[a] = true
	}

	return &podAnnotatorHandler{Client: client, annotations: annotationsMap}
}

func (a *podAnnotatorHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := a.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	namespace := corev1.Namespace{}
	if err := a.Client.Get(ctx, types.NamespacedName{Name: pod.Namespace}, &namespace); err != nil {
		return admission.Errored(http.StatusBadRequest, errors.Wrapf(err, "failed to get namespace %s", pod.Namespace))
	}

	for k, v := range namespace.Annotations {
		if _, f := a.annotations[k]; f { // if annotation is whitelisted
			if _, podHasAnnotation := pod.Annotations[k]; !podHasAnnotation { // if pod already has annotation, don't inherit
				pod.Annotations[k] = v
			}
		}
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func (a *podAnnotatorHandler) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
