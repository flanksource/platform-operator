package test

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/flanksource/commons/utils"
	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = XDescribe("ClusterResourceQuota Controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	var clusterResourceQuota *platformv1.ClusterResourceQuota
	var resourceQuotas = []v1.ResourceQuota{}
	var namespaces = []v1.Namespace{}
	var log = logf.Log.WithName("ClusterResourceQuota test")

	AfterEach(func() {
		if clusterResourceQuota != nil {
			err := k8sClient.Delete(context.Background(), clusterResourceQuota)
			if err != nil {
				log.Error(err, "Failed to delete cluster resource quota")
			}
		}
		for _, r := range resourceQuotas {
			if err := k8sClient.Delete(context.Background(), &r); err != nil {
				log.Error(err, "Failed to delete resource quota", "resourcequota", r.Name)
			}
		}
		for _, n := range namespaces {
			if err := k8sClient.Delete(context.Background(), &n); err != nil {
				log.Error(err, "Failed to delete namespace", "namespace", n.Name)
			}
		}
	})

	Context("ClusterResourceQuota exists", func() {
		It("allows ResourceQuota creation within limits", func() {
			n1 := v1.Namespace{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ns-with-annotations-%s", utils.RandomString(3))},
			}
			n2 := v1.Namespace{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ns-with-annotations-%s", utils.RandomString(3))},
			}
			namespaces = append(namespaces, n1, n2)
			err := k8sClient.Create(context.Background(), &n1)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Create(context.Background(), &n2)
			Expect(err).ToNot(HaveOccurred())

			crq := platformv1.ClusterResourceQuota{
				TypeMeta:   metav1.TypeMeta{APIVersion: "platform.flanksource.com/v1", Kind: "ClusterResourceQuota"},
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("crq-%s", utils.RandomString(3))},
				Spec: platformv1.ClusterResourceQuotaSpec{
					Quota: v1.ResourceQuotaSpec{
						Hard: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("2"),
							v1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			}
			err = k8sClient.Create(context.Background(), &crq)
			Expect(err).ToNot(HaveOccurred())

			r1 := v1.ResourceQuota{
				TypeMeta:   metav1.TypeMeta{"v1", "ResourceQuota"},
				ObjectMeta: metav1.ObjectMeta{Name: "rq", Namespace: n1.Name},
				Spec: v1.ResourceQuotaSpec{
					Hard: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("3"),
						v1.ResourceMemory: resource.MustParse("1500Mi"),
					},
				},
			}
			err = k8sClient.Create(context.Background(), &r1)
			Expect(err).ToNot(HaveOccurred())

			r2 := v1.ResourceQuota{
				TypeMeta:   metav1.TypeMeta{"v1", "ResourceQuota"},
				ObjectMeta: metav1.ObjectMeta{Name: "rq", Namespace: n2.Name},
				Spec: v1.ResourceQuotaSpec{
					Hard: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("1"),
						v1.ResourceMemory: resource.MustParse("600Mi"),
					},
				},
			}
			err = k8sClient.Create(context.Background(), &r2)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("ClusterResourceQuota exists", func() {
		It("allows ResourceQuota creation within limits", func() {
			rq := v1.ResourceQuotaList{}
			err := k8sClient.List(context.Background(), &rq)
			Expect(err).ToNot(HaveOccurred())

			// create two resource quotas
			// check if cluster resource quota can be created only withing limits
			// TODO: currently we can't delete resource quotas created by previous tests
			// Using testEnv resources cannot be properly deleted unfortunately
		})
	})
})
