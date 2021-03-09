package pod

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type podHandler struct {
	Client client.Client
	*admission.Decoder
	Log logr.Logger
	platformv1.PodMutaterConfig
}

func NewMutatingWebhook(client client.Client, cfg platformv1.PodMutaterConfig) *admission.Webhook {
	cfg.AnnotationsMap = make(map[string]bool)
	for _, a := range cfg.Annotations {
		cfg.AnnotationsMap[a] = true
	}
	decoder, _ := admission.NewDecoder(client.Scheme())
	return &admission.Webhook{
		Handler: &podHandler{
			Decoder:          decoder,
			Client:           client,
			PodMutaterConfig: cfg,
			Log:              logf.Log.WithName("pod-mutator")},
	}
}

// +kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=ignore,groups="",resources=pods,verbs=create;update,versions=v1,name=annotate-pods-v1.platform.flanksource.com
func (handler *podHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := handler.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	namespace := corev1.Namespace{}
	if err := handler.Client.Get(ctx, types.NamespacedName{Name: req.Namespace}, &namespace); err != nil {
		return admission.Errored(http.StatusBadRequest, errors.Wrapf(err, "failed to get namespace %s", pod.Namespace))
	}

	pod = handler.UpdatePod(namespace, pod)

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, errors.Wrapf(err, "Failed to marshal pod"))
	}
	response := admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
	return response
}

func (handler *podHandler) UpdateContainer(container v1.Container) v1.Container {
	whitelisted := false
	for _, reg := range handler.RegistryWhitelist {
		if strings.HasPrefix(container.Image, reg) {
			whitelisted = true
			break
		}
	}
	if !whitelisted {
		to := fmt.Sprintf("%s/%s", handler.DefaultRegistryPrefix, container.Image)
		handler.Log.Info("Updating image", "from", container.Image, "to", to)
		container.Image = to
	}

	return container
}

func (handler *podHandler) UpdatePod(namespace v1.Namespace, pod *v1.Pod) *v1.Pod {
	pod, _ = UpdateAnnotations(namespace, handler.PodMutaterConfig, pod)
	pod = handler.UpdateSecrets(pod)
	pod = handler.UpdateTolerations(namespace, pod)
	pod.Spec.Containers = handler.UpdateContainers(pod.Spec.Containers)
	pod.Spec.InitContainers = handler.UpdateContainers(pod.Spec.InitContainers)
	return pod
}

func (handler *podHandler) UpdateTolerations(namespace v1.Namespace, pod *v1.Pod) *v1.Pod {
	tolerations := pod.Spec.Tolerations
	tolerationsValue := namespace.GetAnnotations()[handler.PodMutaterConfig.TolerationsAnnotation]
	for _, toleration := range strings.Split(tolerationsValue, ";") {
		if toleration == "" {
			continue
		}
		key := strings.Split(toleration, "=")[0]
		value := strings.Split(toleration, "=")[1]
		value = strings.Split(value, ":")[0]
		handler.Log.Info("Adding toleration", "pod", pod.GetName(), key, value)
		tolerations = append(tolerations, v1.Toleration{
			Key: key, Value: value,
			Effect: v1.TaintEffectNoSchedule,
		})
	}
	pod.Spec.Tolerations = tolerations
	return pod
}

func (handler *podHandler) UpdateSecrets(pod *v1.Pod) *v1.Pod {
	if len(pod.Spec.ImagePullSecrets) == 0 && handler.DefaultImagePullSecret != "" {
		handler.Log.Info("Injecting image pull secret", "name", handler.DefaultImagePullSecret)
		pod.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{
			Name: handler.DefaultImagePullSecret,
		}}
	}
	return pod
}

func (handler *podHandler) UpdateContainers(containers []v1.Container) []v1.Container {
	_containers := []v1.Container{}
	for _, container := range containers {
		_containers = append(_containers, handler.UpdateContainer(container))
	}
	return _containers
}
