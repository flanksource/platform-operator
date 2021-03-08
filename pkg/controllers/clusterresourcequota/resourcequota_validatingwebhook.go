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
	"sync"

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	corev1 "k8s.io/api/core/v1"
	utilquota "k8s.io/apiserver/pkg/quota/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-resourcequota-v1,mutating=false,failurePolicy=fail,groups="",resources=resourcequotas,verbs=create;update,versions=v1,name=resourcequotas-validation-v1.platform.flanksource.com
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

	if !v.validationEnabled {
		log.Info("validate resource quota flag is not enabled. All requests will be declared valid")
		return admission.Allowed("")
	}

	namespacesList := &corev1.NamespaceList{}
	if err := v.List(ctx, namespacesList); err != nil {
		log.Error(err, "Failed to list namespaces")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// store here the total hard of all resource quotas
	hardTotals := corev1.ResourceList{}
	for _, namespace := range namespacesList.Items {
		namespaceName := namespace.Name

		rqList := &corev1.ResourceQuotaList{}
		if err := v.List(ctx, rqList, client.InNamespace(namespaceName)); err != nil {
			log.Error(err, "Failed to list Resource Quota", "namespace", namespaceName)
			return admission.Errored(http.StatusBadRequest, err)
		}

		if len(rqList.Items) == 0 {
			continue
		}

		for _, rq := range rqList.Items {
			hardTotals = utilquota.Add(hardTotals, rq.Status.Hard)
		}
	}

	hardTotals = utilquota.Add(hardTotals, rq.Spec.Hard)

	// in case in the cluster we define multiple resource quotas
	// NOTE: in the future we could have cluster resource quotas applied to
	//       some namespaces
	quotaList := &platformv1.ClusterResourceQuotaList{}
	if err := v.List(ctx, quotaList); err != nil {
		log.Error(err, "Failed to list cluster resource quotas")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if len(quotaList.Items) == 0 {
		admission.Allowed("")
	}

	for _, q := range quotaList.Items {
		if isOk, rn := utilquota.LessThanOrEqual(hardTotals, q.Spec.Quota.Hard); !isOk {
			return admission.Denied(fmt.Sprintf("resource quota exceeds the cluster resource quota %s in %v", q.Name, rn))
		}
	}
	return admission.Allowed("")
}
