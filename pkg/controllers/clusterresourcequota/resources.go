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
	"sort"

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilquota "k8s.io/apiserver/pkg/quota/v1"
)

func findClusterResourceQuota(ctx context.Context, client client.Client, namespace v1.Namespace) (*platformv1.ClusterResourceQuota, error) {
	quotaList := &platformv1.ClusterResourceQuotaList{}
	err := client.List(ctx, quotaList)
	if err != nil {
		return nil, err
	}
	for _, quota := range quotaList.Items {
		if matches(namespace, &quota) {
			return &quota, nil
		}
	}

	return nil, nil
}

// findMatchingResourceQuotas returns all resource quotas matched by a cluster resource quota
// excluding the resource quota being worked on
func findMatchingResourceQuotas(ctx context.Context, c client.Client, crq *platformv1.ClusterResourceQuota, existing *corev1.ResourceQuota) ([]corev1.ResourceQuota, error) {
	var namespaces v1.NamespaceList
	if err := c.List(ctx, &namespaces, client.MatchingLabels(crq.Spec.MatchLabels)); err != nil {
		return nil, err
	}

	resources := []corev1.ResourceQuota{}

	for _, namespace := range namespaces.Items {
		namespaceName := namespace.Name
		rqList := &corev1.ResourceQuotaList{}
		if err := c.List(ctx, rqList, client.InNamespace(namespaceName)); err != nil {
			return nil, err
		}

		if len(rqList.Items) == 0 {
			continue
		}

		for _, item := range rqList.Items {
			if existing != nil && (item.GetNamespace() == existing.GetNamespace() && item.GetName() == existing.GetName()) {
				continue
			}
			resources = append(resources, item)
		}

	}
	return resources, nil
}

func sumByNamespace(list []corev1.ResourceQuota) (map[string]corev1.ResourceQuota, []string) {
	sum := map[string]corev1.ResourceQuota{}
	keys := []string{}
	for _, item := range list {

		if _, ok := sum[item.GetNamespace()]; !ok {
			keys = append(keys, item.GetNamespace())
			sum[item.GetNamespace()] = corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{Namespace: item.GetNamespace()},
				Spec:       v1.ResourceQuotaSpec{},
				Status:     v1.ResourceQuotaStatus{},
			}
		}
		resource := sum[item.GetNamespace()]
		resource.Spec.Hard = utilquota.Add(resource.Spec.Hard, item.Spec.Hard)
		resource.Status.Used = utilquota.Add(resource.Status.Used, item.Status.Used)
	}
	sort.Strings(keys)
	return sum, keys
}

func sumOfHard(list []corev1.ResourceQuota) corev1.ResourceList {
	sum := corev1.ResourceList{}
	for _, item := range list {
		sum = utilquota.Add(sum, item.Spec.Hard)
	}
	return sum
}

func sumOfUsed(list []corev1.ResourceQuota) corev1.ResourceList {
	sum := corev1.ResourceList{}
	for _, item := range list {
		sum = utilquota.Add(sum, item.Status.Used)
	}
	return sum
}

func namespaceKey(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{
		Name: obj.GetNamespace(),
	}
}

// greaterThan returns true if a > b for any key in b
// If false, it returns the keys in a that exceeded b
func greaterThan(a corev1.ResourceList, b corev1.ResourceList) (bool, []corev1.ResourceName) {
	result := true
	resourceNames := []corev1.ResourceName{}
	for key, value := range b {
		if other, found := a[key]; found {
			if other.Cmp(value) == 1 {
				result = false
				resourceNames = append(resourceNames, key)
			}
		}
	}
	return result, resourceNames
}

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
