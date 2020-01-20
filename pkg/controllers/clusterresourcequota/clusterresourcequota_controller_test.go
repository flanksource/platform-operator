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
	"reflect"
	"testing"

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func setupScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	if err := platformv1.AddToScheme(scheme); err != nil {
		panic(err)
	}

	if err := corev1.AddToScheme(scheme); err != nil {
		panic(err)
	}

	return scheme
}

func TestReconcileClusterResourceQuota_doReconcile(t *testing.T) {
	testCases := []struct {
		name string
		cr   *platformv1.ClusterResourceQuota
		// external objects like Resource Quota
		objects           []runtime.Object
		quotaStatusWanted platformv1.ClusterResourceQuotaStatus
	}{
		{
			name: "if no resource quota is present in any namespace, return status per namespace empty",
			cr: &platformv1.ClusterResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name: "clusterresourcequota-sample",
				},
				Spec: platformv1.ClusterResourceQuotaSpec{
					Quota: corev1.ResourceQuotaSpec{
						Hard: corev1.ResourceList{
							corev1.ResourcePods: resource.MustParse("10"),
						},
					},
				},
			},
			objects: []runtime.Object{},
			quotaStatusWanted: platformv1.ClusterResourceQuotaStatus{
				Total: corev1.ResourceQuotaStatus{
					Hard: corev1.ResourceList{
						corev1.ResourcePods: resource.MustParse("10"),
					},
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// TODO: @mazzy89 Refactor in favor of pkg/eventest
			// Deprecated: This package will be dropped before the v1.0.0 release. Package fake provides a fake client for testing.
			k8sFakeClient := fake.NewFakeClientWithScheme(setupScheme(), append(tt.objects, tt.cr)...)
			r := &ReconcileClusterResourceQuota{
				Client: k8sFakeClient,
				Scheme: setupScheme(),
			}

			request := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: tt.cr.GetNamespace(),
					Name:      tt.cr.GetName(),
				},
			}

			result, err := r.Reconcile(request)
			if err != nil {
				t.Error(err)
			}

			if result.Requeue != false {
				t.Errorf("Expected no requeue, got %v", result.Requeue)
			}

			quota := &platformv1.ClusterResourceQuota{}
			if err := r.Client.Get(context.TODO(), types.NamespacedName{
				Namespace: tt.cr.GetNamespace(),
				Name:      tt.cr.GetName(),
			}, quota); err != nil {
				t.Error(err)
			}

			if !reflect.DeepEqual(quota.Status, tt.quotaStatusWanted) {
				t.Errorf("Expected quota status %v got %v", tt.quotaStatusWanted, quota.Status)
			}
		})
	}
}
