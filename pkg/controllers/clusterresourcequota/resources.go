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
	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"

	corev1 "k8s.io/api/core/v1"
)

func GetResourceQuotasStatusByNamespace(namespaceStatuses platformv1.ResourceQuotasStatusByNamespace, namespace string) corev1.ResourceQuotaStatus {
	for i := range namespaceStatuses {
		curr := namespaceStatuses[i]
		if curr.Namespace == namespace {
			return curr.Status
		}
	}

	return corev1.ResourceQuotaStatus{}
}

func InsertResourceQuotasStatus(namespaceStatuses *platformv1.ResourceQuotasStatusByNamespace, newStatus platformv1.ResourceQuotaStatusByNamespace) {
	newNamespaceStatuses := platformv1.ResourceQuotasStatusByNamespace{}
	found := false

	for i := range *namespaceStatuses {
		curr := (*namespaceStatuses)[i]
		if curr.Namespace == newStatus.Namespace {
			// do this so that we don't change serialization order
			newNamespaceStatuses = append(newNamespaceStatuses, newStatus)
			found = true
			continue
		}
		newNamespaceStatuses = append(newNamespaceStatuses, curr)
	}

	if !found {
		newNamespaceStatuses = append(newNamespaceStatuses, newStatus)
	}

	*namespaceStatuses = newNamespaceStatuses
}

func RemoveResourceQuotasStatusByNamespace(namespaceStatuses *platformv1.ResourceQuotasStatusByNamespace, namespace string) {
	newNamespaceStatuses := platformv1.ResourceQuotasStatusByNamespace{}
	for i := range *namespaceStatuses {
		curr := (*namespaceStatuses)[i]
		if curr.Namespace == namespace {
			continue
		}
		newNamespaceStatuses = append(newNamespaceStatuses, curr)
	}
	*namespaceStatuses = newNamespaceStatuses
}
