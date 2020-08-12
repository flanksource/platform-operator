package v1

import (
	"context"
	"encoding/json"
	"net/http"

	utilsk8s "github.com/flanksource/platform-operator/pkg/controllers/utils/k8s"
	"k8s.io/api/extensions/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var ilog = logf.Log.WithName("ingress-annotator")

func IngressAnnotatorMutateWebhook(client client.Client, svcName, svcNamespace, domain string) *admission.Webhook {
	return &admission.Webhook{
		Handler: NewIngressAnnotatorHandler(client, svcName, svcNamespace, domain),
	}
}

type ingressAnnotatorHandler struct {
	Client                  client.Client
	decoder                 *admission.Decoder
	ingressAnnotator        *utilsk8s.IngressAnnotator
	ingressAnnotatorEnabled bool
}

// +kubebuilder:webhook:path=/mutate-v1-ingress,mutating=true,failurePolicy=ignore,groups="extensions",resources=ingresses,verbs=create;update,versions=v1beta1,name=annotate-ingress-v1.platform.flanksource.com

func NewIngressAnnotatorHandler(client client.Client, svcName, svcNamespace, domain string) *ingressAnnotatorHandler {
	var ingressAnnotator *utilsk8s.IngressAnnotator = nil
	var ingressAnnotatorEnabled = false

	if svcName != "" && svcNamespace != "" {
		ingressAnnotator = utilsk8s.NewIngressAnnotator(client, svcName, svcNamespace, domain)
		ingressAnnotatorEnabled = true
	}

	return &ingressAnnotatorHandler{Client: client, ingressAnnotator: ingressAnnotator, ingressAnnotatorEnabled: ingressAnnotatorEnabled}
}

func (a *ingressAnnotatorHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	ingress := &v1beta1.Ingress{}
	err := a.decoder.Decode(req, ingress)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !a.ingressAnnotatorEnabled {
		ilog.Info("ingress annotator is not enabled. All requests will be declared valid")
		return admission.Allowed("")
	}

	newIngress, changed, err := a.ingressAnnotator.Annotate(ctx, ingress)
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

func (a *ingressAnnotatorHandler) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
