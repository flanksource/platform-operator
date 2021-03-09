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

func createAndFetchPod(namespace string, pod v1.Pod) v1.Pod {
	pod.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}
	pod.ObjectMeta = metav1.ObjectMeta{
		Name:      fmt.Sprintf("pod-%s", utils.RandomString(6)),
		Namespace: namespace,
	}

	if len(pod.Spec.Containers) == 0 {
		pod.Spec.Containers = []v1.Container{
			{
				Name:  "busybox",
				Image: "busybox:latest",
			},
		}
	}
	err := k8sClient.Create(context.Background(), &pod)
	Expect(err).ToNot(HaveOccurred())

	time.Sleep(time.Second * 5)

	key := types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}
	fetched := v1.Pod{}
	err = k8sClient.Get(context.Background(), key, &fetched)
	Expect(err).ToNot(HaveOccurred())
	return fetched
}

var busybox = []v1.Container{
	{
		Name:  "busybox",
		Image: "busybox:latest",
	},
}

var _ = Describe("Pod Controller", func() {
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
					"tolerations":         "node.kubernetes.io/group=instrumented:NoSchedule",
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

	Context("A pod with with a whitelisted image url", func() {
		It("Should leave the image path the same", func() {
			pod := createAndFetchPod(namespace2.Name, v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "busybox",
							Image: "whitelist/busybox:latest",
						},
					},
				},
			})
			Expect(pod.Spec.Containers[0].Image).To(Equal("whitelist/busybox:latest"))

		})
	})

	Context("A pod with with a non-whitelisted image url", func() {
		It("Should prefix the image path ", func() {
			pod := createAndFetchPod(namespace2.Name, v1.Pod{})
			Expect(pod.Spec.Containers[0].Image).To(Equal("registry.cluster.local/busybox:latest"))
		})
	})

	Context("A pod with a namespaced toleration", func() {
		It("Should have a matching toleration", func() {
			pod := createAndFetchPod(namespace2.Name, v1.Pod{})
			if len(pod.Spec.Tolerations) == 0 {
				Fail("no toleration not found")
			} else {
				Expect(pod.Spec.Tolerations[0].Key).To(Equal("node.kubernetes.io/group"))
				Expect(pod.Spec.Tolerations[0].Value).To(Equal("instrumented"))
				Expect(pod.Spec.Tolerations[0].Effect).To(Equal(v1.TaintEffectNoSchedule))
			}
		})
	})

	Context("Namespace with annotations", func() {
		It("Should add annotations to pods without annotation", func() {
			pod := &v1.Pod{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("pod-without-annotations-%s", utils.RandomString(6)),
					Namespace: namespace1.Name,
				},
				Spec: v1.PodSpec{
					Containers: busybox,
				},
			}

			err := k8sClient.Create(context.Background(), pod)
			Expect(err).ToNot(HaveOccurred())

			key := types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}
			fetched := &v1.Pod{}
			Eventually(func() string {
				err := k8sClient.Get(context.Background(), key, fetched)
				if err != nil {
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
