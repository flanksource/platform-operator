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
	"github.com/flanksource/platform-operator/pkg/controllers/podannotator"
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
	var oauth2ProxySvcName string
	var oauth2ProxySvcNamespace string
	var domain string
	var registryWhitelist string
	var annotations string
	cfg := platformv1.PodMutaterConfig{}

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")

	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	flag.DurationVar(&cleanupInterval, "cleanup-interval", 10*time.Minute, "Frequency at which the cleanup controller runs.")
	flag.DurationVar(&annotationInterval, "annotation-interval", 10*time.Minute, "Frequency at which the annotation controller runs.")

	flag.StringVar(&annotations, "annotations", "", "Annotations pods inherit from parent namespace")
	flag.BoolVar(&enableClusterResourceQuota, "enable-cluster-resource-quota", true, "Enable/Disable cluster resource quota")

	flag.StringVar(&oauth2ProxySvcName, "oauth2-proxy-service-name", "", "Name of oauth2-proxy service")
	flag.StringVar(&oauth2ProxySvcNamespace, "oauth2-proxy-service-namespace", "", "Name of oauth2-proxy service namespace")
	flag.StringVar(&cfg.DefaultRegistryPrefix, "default-registry-prefix", "", "A default registry prefix path to apply to all pods")
	flag.StringVar(&cfg.DefaultImagePullSecret, "default-image-pull-secret", "", "Default dmage pull secret to apply to all pods")
	flag.StringVar(&registryWhitelist, "registry-whitelist", "", "A list of image prefixes to ignore")
	flag.StringVar(&domain, "domain", "", "Domain used by platform")

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

	// TODO(mazzy89): Make the adding of controllers more dynamic

	if err := cleanup.Add(mgr, cleanupInterval); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Cleanup")
		os.Exit(1)
	}

	if enableClusterResourceQuota {
		if err := clusterresourcequota.Add(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "ClusterResourceQuota")
			os.Exit(1)
		}
	}

	if err := podannotator.Add(mgr, annotationInterval, cfg); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PodAnnotator")
		os.Exit(1)
	}

	if oauth2ProxySvcName != "" && oauth2ProxySvcNamespace != "" {
		if err := ingress.Add(mgr, annotationInterval, oauth2ProxySvcName, oauth2ProxySvcNamespace, domain); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "IngressAnnotator")
			os.Exit(1)
		}
	}

	// Setup webhooks
	setupLog.Info("setting up webhook server")
	hookServer := mgr.GetWebhookServer()

	mtx := &sync.Mutex{}

	setupLog.Info("registering webhooks to the webhook server")
	hookServer.Register("/mutate-v1-pod", &webhook.Admission{Handler: platformv1.PodAnnotatorMutateWebhook(mgr.GetClient(), cfg)})
	hookServer.Register("/mutate-v1-ingress", &webhook.Admission{Handler: platformv1.IngressAnnotatorMutateWebhook(mgr.GetClient(), oauth2ProxySvcName, oauth2ProxySvcNamespace, domain)})
	hookServer.Register("/validate-clusterresourcequota-platform-flanksource-com-v1", platformv1.ClusterResourceQuotaValidatingWebhook(mtx, enableClusterResourceQuota))
	hookServer.Register("/validate-resourcequota-v1", platformv1.ResourceQuotaValidatingWebhook(mtx, enableClusterResourceQuota))

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
