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

package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"sync"
	"time"

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	"github.com/flanksource/platform-operator/pkg/controllers/cleanup"
	"github.com/flanksource/platform-operator/pkg/controllers/clusterresourcequota"
	"github.com/flanksource/platform-operator/pkg/controllers/ingress"
	"github.com/flanksource/platform-operator/pkg/controllers/pod"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = logf.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = platformv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool

	var cleanupInterval, annotationInterval time.Duration
	var enableClusterResourceQuota bool
	var ingressSSO bool
	var oauth2ProxySvcName string
	var oauth2ProxySvcNamespace string
	var domain string

	var registryWhitelist string
	var annotations string
	var podMutator bool
	cfg := platformv1.PodMutaterConfig{}

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")

	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	flag.DurationVar(&cleanupInterval, "cleanup-interval", 10*time.Minute, "Frequency at which the cleanup controller runs.")
	flag.DurationVar(&annotationInterval, "annotation-interval", 10*time.Minute, "Frequency at which the annotation controller runs.")

	flag.BoolVar(&enableClusterResourceQuota, "enable-cluster-resource-quota", true, "Enable/Disable cluster resource quota")

	flag.BoolVar(&ingressSSO, "enable-ingress-sso", false, "Enable ingress mutation hook for restrict-to-groups SSO")
	flag.StringVar(&oauth2ProxySvcName, "oauth2-proxy-service-name", "", "Name of oauth2-proxy service")
	flag.StringVar(&oauth2ProxySvcNamespace, "oauth2-proxy-service-namespace", "", "Name of oauth2-proxy service namespace")
	flag.StringVar(&domain, "domain", "", "Domain used by platform")

	flag.BoolVar(&podMutator, "enable-pod-mutations", true, "Enable pod mutating webhooks")

	flag.StringVar(&annotations, "annotations", "", "Annotations pods inherit from parent namespace")
	flag.StringVar(&cfg.DefaultRegistryPrefix, "default-registry-prefix", "", "A default registry prefix path to apply to all pods")
	flag.StringVar(&cfg.DefaultImagePullSecret, "default-image-pull-secret", "", "A default image pull secret to apply to all pods")
	flag.StringVar(&registryWhitelist, "registry-whitelist", "", "A list of image prefixes to ignore")
	flag.StringVar(&cfg.TolerationsPrefix, "namespace-tolerations-prefix", "tolerations/", "A prefix for namespace level annotations that should be applied as tolerations on pods")
	flag.Parse()

	cfg.Annotations = strings.Split(annotations, ",")
	cfg.RegistryWhitelist = strings.Split(registryWhitelist, ",")

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup webhooks
	setupLog.Info("setting up webhook server")
	hookServer := mgr.GetWebhookServer()

	mtx := &sync.Mutex{}

	if err := cleanup.Add(mgr, cleanupInterval); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Cleanup")
		os.Exit(1)
	}

	if enableClusterResourceQuota {
		if err := clusterresourcequota.Add(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ClusterResourceQuota")
			os.Exit(1)
		}
		hookServer.Register("/validate-clusterresourcequota-platform-flanksource-com-v1", clusterresourcequota.NewClusterResourceQuotaValidatingWebhook(mgr.GetClient(), mtx, enableClusterResourceQuota))
		hookServer.Register("/validate-resourcequota-v1", clusterresourcequota.NewResourceQuotaValidatingWebhook(mgr.GetClient(), mtx, enableClusterResourceQuota))

	}

	if podMutator {
		if err := pod.Add(mgr, annotationInterval, cfg); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "PodAnnotator")
			os.Exit(1)
		}
		hookServer.Register("/mutate-v1-pod", &webhook.Admission{Handler: pod.NewMutatingWebhook(mgr.GetClient(), cfg)})
	}

	if ingressSSO {
		if err := ingress.Add(mgr, annotationInterval, oauth2ProxySvcName, oauth2ProxySvcNamespace, domain); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "IngressAnnotator")
			os.Exit(1)
		}
		hookServer.Register("/mutate-v1-ingress", &webhook.Admission{Handler: ingress.NewMutatingWebhook(mgr.GetClient(), oauth2ProxySvcName, oauth2ProxySvcNamespace, domain)})
	}

	// +kubebuilder:scaffold:builder

	go func() {
		setupLog.Info("Starting profiling server on localhost:6060")
		setupLog.Error(http.ListenAndServe("0.0.0.0:6060", nil), "problem starting pprof server")
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
