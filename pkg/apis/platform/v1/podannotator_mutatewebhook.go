package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type PodMutaterConfig struct {
	AnnotationsMap         map[string]bool
	Annotations            []string
	RegistryWhitelist      []string
	DefaultRegistryPrefix  string
	DefaultImagePullSecret string
}

func PodAnnotatorMutateWebhook(client client.Client, cfg PodMutaterConfig) *admission.Webhook {
	return &admission.Webhook{
		Handler: NewPodAnnotatorHandler(client, cfg),
	}
}

type podAnnotatorHandler struct {
	Client  client.Client
	decoder *admission.Decoder
	Log     logr.Logger
	PodMutaterConfig
}

// +kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=ignore,groups="",resources=pods,verbs=create;update,versions=v1,name=annotate-pods-v1.platform.flanksource.com
func NewPodAnnotatorHandler(client client.Client, cfg PodMutaterConfig) *podAnnotatorHandler {
	cfg.AnnotationsMap = make(map[string]bool)
	for _, a := range cfg.Annotations {
		cfg.AnnotationsMap[a] = true
	}
	return &podAnnotatorHandler{Client: client, PodMutaterConfig: cfg, Log: logf.Log.WithName("pod-mutator")}
}

func (a *podAnnotatorHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := a.decoder.Decode(req, pod)
	a.Log.Info("Mutating", "pod", pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	namespace := corev1.Namespace{}
	if err := a.Client.Get(ctx, types.NamespacedName{Name: req.Namespace}, &namespace); err != nil {
		return admission.Errored(http.StatusBadRequest, errors.Wrapf(err, "failed to get namespace %s", pod.Namespace))
	}

	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}

	if namespace.Annotations == nil {
		namespace.Annotations = map[string]string{}
	}

	for k, v := range namespace.Annotations {
		if _, f := a.AnnotationsMap[k]; f { // if annotation is whitelisted
			if _, podHasAnnotation := pod.Annotations[k]; !podHasAnnotation { // if pod already has annotation, don't inherit
				pod.Annotations[k] = v
			}
		}
	}

containers:
	for _, container := range pod.Spec.Containers {
		for _, reg := range a.RegistryWhitelist {
			if strings.HasPrefix(container.Image, reg) {
				continue containers
			}
		}
		to := fmt.Sprintf("%s/%s", a.DefaultRegistryPrefix, container.Image)
		a.Log.V(2).Info("Updating image", "from", container.Image, "to", to)
		container.Image = to
	}

initContainers:
	for _, container := range pod.Spec.InitContainers {
		for _, reg := range a.RegistryWhitelist {
			if strings.HasPrefix(container.Image, reg) {
				continue initContainers
			}
		}
		to := fmt.Sprintf("%s/%s", a.DefaultRegistryPrefix, container.Image)
		a.Log.V(2).Info("Updating image", "from", container.Image, "to", to)
		container.Image = to
		container.Image = fmt.Sprintf("%s/%s", a.DefaultRegistryPrefix, container.Image)
	}

	if len(pod.Spec.ImagePullSecrets) == 0 && a.DefaultImagePullSecret != "" {
		a.Log.V(2).Info("Injecting image pull secret", "name", a.DefaultImagePullSecret)
		pod.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{
			Name: a.DefaultImagePullSecret,
		}}
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
