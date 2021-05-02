/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clusterresourcequota

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	utilquota "k8s.io/apiserver/pkg/quota/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-resourcequota-v1,mutating=false,sideEffects=None,admissionReviewVersions=v1,failurePolicy=fail,groups="",resources=resourcequotas,verbs=create;update,versions=v1,name=resourcequotas-validation-v1.platform.flanksource.com
func NewResourceQuotaValidatingWebhook(client client.Client, mtx *sync.Mutex, validationEnabled bool) *admission.Webhook {
	decoder, _ := admission.NewDecoder(client.Scheme())
	return &admission.Webhook{
		Handler: &validatingResourceQuotaHandler{
			Client:            client,
			Decoder:           decoder,
			mtx:               mtx,
			validationEnabled: validationEnabled},
	}
}

type validatingResourceQuotaHandler struct {
	client.Client
	*admission.Decoder
	mtx               *sync.Mutex
	validationEnabled bool
}

var _ admission.Handler = &validatingResourceQuotaHandler{}

func (v *validatingResourceQuotaHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	v.mtx.Lock()
	defer v.mtx.Unlock()

	rq := &corev1.ResourceQuota{}

	if err := v.Decode(req, rq); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	var namespace v1.Namespace
	if err := v.Client.Get(ctx, namespaceKey(rq), &namespace); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("cannot find namespace for resource quota: %s", rq.Namespace))
	}

	crq, err := findClusterResourceQuota(ctx, v.Client, namespace)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if crq == nil {
		return admission.Allowed("")
	}

	if !v.validationEnabled {
		log.Info("validate resource quota flag is not enabled. All requests will be declared valid")
		return admission.Allowed("")
	}

	existing, err := findMatchingResourceQuotas(ctx, v.Client, crq, rq)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	sum := sumOfHard(append(existing, *rq))

	if isOk, rn := utilquota.LessThanOrEqual(sum, crq.Spec.Hard); !isOk {
		msg := ""
		for _, resource := range rn {
			msg += fmt.Sprintf(" %s(%s > %s)", resource, qtyString(sum[resource]), qtyString(crq.Spec.Hard[resource]))
		}
		return admission.Denied(fmt.Sprintf("ResourceQuota/%s/%s would exceed ClusterResourceQuota/%s: %s", rq.Namespace, rq.Name, crq.Name, strings.TrimSpace(msg)))
	}
	return admission.Allowed("")
}
