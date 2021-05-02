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
)

var matchBy = map[string]string{"name": "n1"}

func newClusterResourceQuota() *platformv1.ClusterResourceQuota {
	return &platformv1.ClusterResourceQuota{
		TypeMeta:   metav1.TypeMeta{APIVersion: "platform.flanksource.com/v1", Kind: "ClusterResourceQuota"},
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("crq-%s", utils.RandomString(3))},
		Status: platformv1.ClusterResourceQuotaStatus{
			Namespaces: platformv1.ResourceQuotasStatusByNamespace{},
		},
		Spec: platformv1.ClusterResourceQuotaSpec{
			MatchLabels: matchBy,
			ResourceQuotaSpec: v1.ResourceQuotaSpec{
				Hard: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("2"),
					v1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
		},
	}
}

var _ = Describe("ClusterResourceQuota Controller", func() {

	var ctx = context.Background()
	const timeout = time.Second * 30
	const interval = time.Second * 1
	n1 := v1.Namespace{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ns-with-clusterquota-%s", utils.RandomString(3)), Labels: matchBy},
	}

	n2 := v1.Namespace{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("ns-with-clusterquota-%s", utils.RandomString(3)), Labels: matchBy},
	}

	var crq *platformv1.ClusterResourceQuota

	CreateQuota := func(namespace, cpu, memory string) (v1.ResourceQuota, error) {
		r := v1.ResourceQuota{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ResourceQuota"},
			ObjectMeta: metav1.ObjectMeta{Name: "rq", Namespace: namespace},
			Spec: v1.ResourceQuotaSpec{
				Hard: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(cpu),
					v1.ResourceMemory: resource.MustParse(memory),
				},
			},
		}
		err := k8sClient.Create(ctx, &r)
		return r, err
	}

	Describe("ClusterResourceQuota", func() {

		It("setup", func() {
			err := k8sClient.Create(ctx, &n1)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Create(ctx, &n2)
			Expect(err).ToNot(HaveOccurred())
		})

		BeforeEach(func() {
			crq = newClusterResourceQuota()
			err := k8sClient.Create(ctx, crq)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := k8sClient.Delete(ctx, crq)
			Expect(err).ToNot(HaveOccurred())
			rqList := v1.ResourceQuotaList{}
			err = k8sClient.List(ctx, &rqList)
			Expect(err).ToNot(HaveOccurred())

			for _, rq := range rqList.Items {
				err = k8sClient.Delete(ctx, &rq)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("allows ResourceQuota creation within limits", func() {
			_, err := CreateQuota(n1.Name, "900m", "500Mi")
			Expect(err).ToNot(HaveOccurred())
			_, err = CreateQuota(n2.Name, "1000m", "1Gi")
			Expect(err).ToNot(HaveOccurred())
		})

		XIt("should update its status", func() {
		})

		It("should not allow updating to lower than ResourceQuota", func() {
			_, err := CreateQuota(n1.Name, "1100m", "1Gi")
			Expect(err).ToNot(HaveOccurred())
			crq.Spec.Hard = v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("500m"),
				v1.ResourceMemory: resource.MustParse("2Gi"),
			}
			err = k8sClient.Update(ctx, crq)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cpu(1100m > 500m)"))
		})

		It("should allow updating to higher than ResourceQuota", func() {
			_, err := CreateQuota(n1.Name, "1000m", "1Gi")
			Expect(err).ToNot(HaveOccurred())
			crq.Spec.Hard = v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("1500m"),
				v1.ResourceMemory: resource.MustParse("2Gi"),
			}
			err = k8sClient.Update(ctx, crq)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not allow ResourceQuota creation outside of limits", func() {
			_, err := CreateQuota(n1.Name, "1000m", "1Gi")
			Expect(err).ToNot(HaveOccurred())
			_, err = CreateQuota(n2.Name, "1500m", "1Gi")
			Expect(err).To(HaveOccurred())
		})
	})
})
