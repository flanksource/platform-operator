package test

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/flanksource/commons/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Cleanup Controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	const cleanupLabel = "auto-delete"
	var namespace1, namespace2 *v1.Namespace

	BeforeEach(func() {
		namespace1 = &v1.Namespace{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("namespace-to-delete-%s", utils.RandomString(3)),
				Labels: map[string]string{
					"auto-delete": "5s",
				},
			},
		}
		namespace2 = &v1.Namespace{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("namespace-to-not-delete-%s", utils.RandomString(3)),
			},
		}
		err := k8sClient.Create(context.Background(), namespace1)
		Expect(err).ToNot(HaveOccurred())
		err = k8sClient.Create(context.Background(), namespace2)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		k8sClient.Delete(context.Background(), namespace1)
		k8sClient.Delete(context.Background(), namespace2)
	})

	Context("Namespace with label", func() {
		It("Should remove namespace created with auto-delete tag", func() {
			key := types.NamespacedName{Name: namespace1.Name}

			// First check that the namespace exists
			ns := &v1.Namespace{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), key, ns)
			}, 5*time.Second, 1*time.Second).Should(Not(HaveOccurred()))

			time.Sleep(time.Second * 10)

			err := k8sClient.Get(context.Background(), key, ns)
			Expect(err).ToNot(HaveOccurred())

			fetched := &v1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), key, fetched)
				if errors.IsNotFound(err) {
					return true
				} else if err != nil {
					fmt.Printf("Error fetching namespace: %v", err)
					return false
				}
				return fetched.Status.Phase == v1.NamespaceTerminating
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Namespace without label", func() {
		It("Should not remove namespace without auto-delete tag", func() {
			key := types.NamespacedName{Name: namespace2.Name}

			// First check that the namespace exists
			ns := &v1.Namespace{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), key, ns)
			}, 5*time.Second, 1*time.Second).Should(Not(HaveOccurred()))

			time.Sleep(time.Second * 5)

			// Check it still exists after 5s
			ns = &v1.Namespace{}
			err := k8sClient.Get(context.Background(), key, ns)
			Expect(err).ToNot(HaveOccurred())
			Expect(ns.Status.Phase).To(Equal(v1.NamespaceActive))
		})

		It("Should remove namespace updated with auto-delete tag", func() {
			key := types.NamespacedName{Name: namespace2.Name}

			// First check that the namespace exists
			ns := &v1.Namespace{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), key, ns)
			}, 5*time.Second, 1*time.Second).Should(Not(HaveOccurred()))

			if ns.Labels == nil {
				ns.Labels = map[string]string{}
			}
			ns.Labels["auto-delete"] = "5s"
			err := k8sClient.Update(context.Background(), ns)
			Expect(err).ToNot(HaveOccurred())

			fetched := &v1.Namespace{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), key, fetched)
				if errors.IsNotFound(err) {
					return true
				} else if err != nil {
					fmt.Printf("Error fetching namespace: %v", err)
					return false
				}
				return fetched.Status.Phase == v1.NamespaceTerminating
			}, timeout, interval).Should(BeTrue())
		})
	})
})
