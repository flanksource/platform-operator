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

package v1

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	corev1 "k8s.io/api/core/v1"
	utilquota "k8s.io/kubernetes/pkg/quota/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var qlog = logf.Log.WithName("clusterresourcequota-validation")

// +kubebuilder:webhook:path=/validate-clusterresourcequota-platform-flanksource-com-v1,mutating=false,failurePolicy=fail,groups=platform.flanksource.com,resources=clusterresourcequotas,verbs=create;update,versions=v1,name=clusterresourcequotas-validation-v1.platform.flanksource.com

func ClusterResourceQuotaValidatingWebhook(mtx *sync.Mutex) *admission.Webhook {
	return &admission.Webhook{
		Handler: &validatingClusterResourceQuotaHandler{mtx: mtx},
	}
}

// ClusterResourceQuotaValidator validates ClusterResourceQuotas
type validatingClusterResourceQuotaHandler struct {
	client  client.Client
	decoder *admission.Decoder
	mtx     *sync.Mutex
}

var _ admission.Handler = &validatingClusterResourceQuotaHandler{}

func (v *validatingClusterResourceQuotaHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	v.mtx.Lock()
	defer v.mtx.Unlock()

	quota := &ClusterResourceQuota{}

	if err := v.decoder.Decode(req, quota); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	namespacesList := &corev1.NamespaceList{}
	if err := v.client.List(ctx, namespacesList); err != nil {
		qlog.Error(err, "Failed to list namespaces")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// store here the total hard of all resource quotas
	hardTotals := corev1.ResourceList{}
	for _, namespace := range namespacesList.Items {
		namespaceName := namespace.Name

		rqList := &corev1.ResourceQuotaList{}
		if err := v.client.List(ctx, rqList, client.InNamespace(namespaceName)); err != nil {
			qlog.Error(err, "Failed to list Resource Quota", "namespace", namespaceName)
			return admission.Errored(http.StatusBadRequest, err)
		}

		if len(rqList.Items) == 0 {
			continue
		}

		for _, rq := range rqList.Items {
			hardTotals = utilquota.Add(hardTotals, rq.Status.Hard)
		}
	}

	// check quota
	if isOk, rn := utilquota.LessThanOrEqual(hardTotals, quota.Spec.Quota.Hard); !isOk {
		return admission.Denied(fmt.Sprintf("total resource quotas exceeed cluster resource quota hard limits %v", rn))
	}

	return admission.Allowed("")
}

// ClusterResourceQuotaValidator implements inject.Client.
// A client will be automatically injected.

// InjectClient injects the client.
func (v *validatingClusterResourceQuotaHandler) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// ClusterResourceQuotaValidator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (v *validatingClusterResourceQuotaHandler) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
