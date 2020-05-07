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
	"os"
	"strings"
	"sync"
	"time"

	platformv1 "github.com/flanksource/platform-operator/pkg/apis/platform/v1"
	"github.com/flanksource/platform-operator/pkg/controllers/cleanup"
	"github.com/flanksource/platform-operator/pkg/controllers/clusterresourcequota"
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
	var annotations string

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")

	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	flag.DurationVar(&cleanupInterval, "cleanup-interval", 10*time.Minute, "Frequency at which the cleanup controller runs.")
	flag.DurationVar(&annotationInterval, "annotation-interval", 10*time.Minute, "Frequency at which the annotation controller runs.")

	flag.StringVar(&annotations, "annotations", "", "Annotations pods inherit from parent namespace")

	flag.Parse()

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

	if err := clusterresourcequota.Add(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterResourceQuota")
		os.Exit(1)
	}

	if err := podannotator.Add(mgr, annotationInterval, strings.Split(annotations, ",")); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PodAnnotator")
		os.Exit(1)
	}

	// Setup webhooks
	setupLog.Info("setting up webhook server")
	hookServer := mgr.GetWebhookServer()

	mtx := &sync.Mutex{}

	setupLog.Info("registering webhooks to the webhook server")
	hookServer.Register("/mutate-v1-pod", &webhook.Admission{Handler: platformv1.PodAnnotatorMutateWebhook(mgr.GetClient(), strings.Split(annotations, ","))})
	hookServer.Register("/validate-clusterresourcequota-platform-flanksource-com-v1", platformv1.ClusterResourceQuotaValidatingWebhook(mtx))
	hookServer.Register("/validate-resourcequota-v1", platformv1.ResourceQuotaValidatingWebhook(mtx))

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
