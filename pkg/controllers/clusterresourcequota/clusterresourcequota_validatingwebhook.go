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

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func NewClusterResourceQuotaValidatingWebhook(client client.Client, mtx *sync.Mutex, validationEnabled bool) *admission.Webhook {
	decoder, _ := admission.NewDecoder(client.Scheme())
	return &admission.Webhook{
		Handler: &validatingClusterResourceQuotaHandler{
			Client:            client,
			Decoder:           decoder,
			mtx:               mtx,
			validationEnabled: validationEnabled},
	}
}

// ClusterResourceQuotaValidator validates ClusterResourceQuotas
type validatingClusterResourceQuotaHandler struct {
	client.Client
	*admission.Decoder
	mtx               *sync.Mutex
	validationEnabled bool
}

var _ admission.Handler = &validatingClusterResourceQuotaHandler{}

func (v *validatingClusterResourceQuotaHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	v.mtx.Lock()
	defer v.mtx.Unlock()

	crq := &platformv1.ClusterResourceQuota{}

	if err := v.Decode(req, crq); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !v.validationEnabled {
		log.Info("validate resource quota flag is not enabled. All requests will be declared valid")
		return admission.Allowed("")
	}

	existing, err := findMatchingResourceQuotas(ctx, v.Client, crq, nil)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	used := sumOfHard(existing)

	if isOk, rn := greaterThan(used, crq.Spec.Hard); !isOk {
		msg := ""
		for _, resource := range rn {
			msg += fmt.Sprintf(" %s(%s > %s)", resource, qtyString(used[resource]), qtyString(crq.Spec.Hard[resource]))
		}
		return admission.Denied(fmt.Sprintf("cannot update ClusterResourceQuota/%s it would be below current usage: %s", crq.Name, strings.TrimSpace(msg)))
	}

	return admission.Allowed("")
}
