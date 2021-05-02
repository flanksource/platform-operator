package ingress

import (
	"context"
	"encoding/json"
	"net/http"

	"k8s.io/api/extensions/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type ingressHandler struct {
	*admission.Decoder
	IngressReconciler
}

func NewMutatingWebhook(client client.Client, svcName, svcNamespace, domain string) *admission.Webhook {
	decoder, _ := admission.NewDecoder(client.Scheme())
	return &admission.Webhook{
		Handler: &ingressHandler{
			Decoder: decoder,
			IngressReconciler: IngressReconciler{
				Client:       client,
				SvcName:      svcName,
				SvcNamespace: svcNamespace,
				Domain:       domain,
			},
		},
	}
}

// +kubebuilder:webhook:path=/mutate-v1-ingress,mutating=true,sideEffects=None,admissionReviewVersions=v1,failurePolicy=ignore,groups="extensions",resources=ingresses,verbs=create;update,versions=v1beta1,name=mutate-ingress-v1.platform.flanksource.com
func (handler *ingressHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	ingress := &v1beta1.Ingress{}

	err := handler.Decode(req, ingress)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	newIngress, changed, err := handler.Annotate(ctx, ingress)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	} else if !changed {
		return admission.Allowed("not changed")
	}

	marshaledIngress, err := json.Marshal(newIngress)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledIngress)
}
