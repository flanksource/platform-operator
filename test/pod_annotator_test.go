package test

import (
	"context"
	"fmt"
	"time"

	"github.com/flanksource/commons/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("PodAnnotator Controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	const annotationName = "foo.example.com/bar"
	var annotationValue = utils.RandomString(10)
	var namespace1, namespace2 *v1.Namespace

	BeforeEach(func() {
		namespace1 = &v1.Namespace{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("ns-with-annotations-%s", utils.RandomString(3)),
				Annotations: map[string]string{
					annotationName:        annotationValue,
					"aaa.example.com/bbb": "42",
				},
			},
		}
		namespace2 = &v1.Namespace{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("ns-without-annotations-%s", utils.RandomString(3)),
				Annotations: map[string]string{
					"aaa.example.com/bbb": "42",
				},
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

	Context("Namespace with annotations", func() {
		It("Should add annotation to pods without annotation", func() {
			pod := &v1.Pod{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("pod-without-annotations-%s", utils.RandomString(6)),
					Namespace: namespace1.Name,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "busybox",
							Image: "busybox:latest",
						},
					},
				},
			}

			err := k8sClient.Create(context.Background(), pod)
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(time.Second * 5)

			key := types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}
			fetched := &v1.Pod{}
			Eventually(func() string {
				err := k8sClient.Get(context.Background(), key, fetched)
				if err != nil {
					fmt.Printf("Error fetching pod: %v", err)
					return ""
				}
				if fetched.Annotations == nil {
					return ""
				}
				return fetched.Annotations[annotationName]
			}, timeout, interval).Should(Equal(annotationValue))
			Expect(fetched.Annotations["aaa.example.com/bbb"]).To(Equal(""))
		})
	})
})
