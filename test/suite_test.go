package test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/flanksource/commons/certs"
	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	"github.com/flanksource/platform-operator/pkg/controllers/cleanup"
	"github.com/flanksource/platform-operator/pkg/controllers/clusterresourcequota"
	"github.com/flanksource/platform-operator/pkg/controllers/pod"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	// +kubebuilder:scaffold:imports
)

var cfg *rest.Config
var k8sClient client.Client
var k8sManager ctrl.Manager
var testEnv *envtest.Environment
var port = 8843

// DefaultKubeAPIServerFlags are default flags necessary to bring up apiserver.
var APIServerFlags = []string{
	// Allow tests to run offline, by preventing API server from attempting to
	// use default route to determine its --advertise-address
	"--advertise-address=127.0.0.1",
	"--etcd-servers={{ if .EtcdURL }}{{ .EtcdURL.String }}{{ end }}",
	"--cert-dir={{ .CertDir }}",
	"--insecure-port={{ if .URL }}{{ .URL.Port }}{{ end }}",
	"--insecure-bind-address={{ if .URL }}{{ .URL.Hostname }}{{ end }}",
	"--secure-port={{ if .SecurePort }}{{ .SecurePort }}{{ end }}",
	"--admission-control=MutatingAdmissionWebhook",
	"--service-cluster-ip-range=10.0.0.0/24",
}

func TestAPIs(t *testing.T) {
	if os.Getenv("TEST_E2E") != "true" {
		return
	}

	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

func waitFor(host string) {
	d := &net.Dialer{Timeout: time.Second}
	Eventually(func() error {
		conn, err := tls.DialWithDialer(d, "tcp", host, &tls.Config{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return err
		}
		_ = conn.Close()
		return nil
	}, 60*time.Second, 1*time.Second).Should(Succeed())
}

func registerWebhook(manager ctrl.Manager, name string, webhook *admission.Webhook, apiGroup, apiVersion string, resources ...string) error {
	wh := &admissionregistrationv1beta1.MutatingWebhookConfiguration{}
	wh.Name = name
	_, err := ctrl.CreateOrUpdate(context.TODO(), manager.GetClient(), wh, func() error {
		failPolicy := admissionregistrationv1beta1.Fail
		caBundle, _ := ioutil.ReadFile("tls.crt")
		urlStr := fmt.Sprintf("https://localhost:%d/%s", port, name)
		wh.Webhooks = []admissionregistrationv1beta1.MutatingWebhook{
			{
				Name:          name,
				FailurePolicy: &failPolicy,
				ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
					CABundle: caBundle,
					URL:      &urlStr,
				},
				Rules: []admissionregistrationv1beta1.RuleWithOperations{
					{
						Operations: []admissionregistrationv1beta1.OperationType{
							admissionregistrationv1beta1.Create, admissionregistrationv1beta1.Update,
						},
						Rule: admissionregistrationv1beta1.Rule{
							APIGroups:   []string{apiGroup},
							APIVersions: []string{apiVersion},
							Resources:   resources,
						},
					},
				},
			},
		}
		return nil
	})

	logf.Log.Info("registering webhooks to the webhook server")
	manager.GetWebhookServer().Register("/"+name, webhook)
	return err

}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:  []string{filepath.Join("..", "config", "crds", "bases")},
		KubeAPIServerFlags: APIServerFlags,
	}

	cfg, err := testEnv.Start()

	logf.Log.Info("started env", "cfg", cfg, "err", err)
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = scheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = platformv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	cert := certs.NewCertificateBuilder("localhost").Certificate
	cert, _ = cert.SignCertificate(cert, 1)
	ioutil.WriteFile("tls.crt", cert.EncodedCertificate(), 0600)
	ioutil.WriteFile("tls.key", cert.EncodedPrivateKey(), 0600)

	// +kubebuilder:scaffold:scheme
	// cwd, _ := os.Getwd()
	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:     false,
		MetricsBindAddress: "0.0.0.0:10250",
		CertDir:            "./",
		Scheme:             scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())
	err = cleanup.Add(k8sManager, 5*time.Second)
	Expect(err).ToNot(HaveOccurred())

	err = clusterresourcequota.Add(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	podConfig := platformv1.PodMutaterConfig{
		Annotations:            []string{"foo.example.com/bar"},
		AnnotationsMap:         map[string]bool{"foo.example.com/bar": true},
		RegistryWhitelist:      []string{"registry.cluster.local", "whitelist"},
		DefaultRegistryPrefix:  "registry.cluster.local",
		TolerationsAnnotation:  "tolerations",
		DefaultImagePullSecret: "registry-secret",
	}
	err = pod.Add(k8sManager, 5*time.Second, podConfig)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		hookServer := k8sManager.GetWebhookServer()
		hookServer.Port = port
		hookServer.Host = "0.0.0.0"
		hookServer.CertDir = "."
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()
	By("Waiting for webhook server to come up")
	waitFor(fmt.Sprintf("localhost:%d", port))
	err = registerWebhook(k8sManager, "annotate-pods-v1.platform.flanksource.com",
		&webhook.Admission{Handler: pod.NewMutatingWebhook(k8sManager.GetClient(), podConfig)},
		"", "v1", "pods")
	Expect(err).ToNot(HaveOccurred())

	err = registerWebhook(k8sManager, "clusterresourcequota-v1.platform.flanksource.com",
		&webhook.Admission{Handler: clusterresourcequota.NewClusterResourceQuotaValidatingWebhook(k8sManager.GetClient(), &sync.Mutex{}, true)},
		"platform.flanksource.com", "v1", "clusterresourcequotas")
	Expect(err).ToNot(HaveOccurred())

	err = registerWebhook(k8sManager, "resourcequota-v1.platform.flanksource.com",
		&webhook.Admission{Handler: clusterresourcequota.NewResourceQuotaValidatingWebhook(k8sManager.GetClient(), &sync.Mutex{}, true)},
		"", "v1", "resourcequotas")
	Expect(err).ToNot(HaveOccurred())
	By("Webhook server is up")
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	gexec.KillAndWait(5 * time.Second)
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
