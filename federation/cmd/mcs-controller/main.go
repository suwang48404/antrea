/*
Copyright 2021 Antrea Authors.

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

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	federationv1alpha1 "github.com/vmware-tanzu/antrea/federation/api/v1alpha1"
	"github.com/vmware-tanzu/antrea/federation/controllers/externalentity"
	_ "github.com/vmware-tanzu/antrea/federation/controllers/externalentity/externalentity_owners"

	"github.com/vmware-tanzu/antrea/federation/controllers/federation"

	"github.com/vmware-tanzu/antrea/federation/webhook"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = federationv1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	// logs.InitLogs() // For klog flags.
	var metricsAddr string
	var enableLeaderElection bool
	var enableDebugLog bool

	flag.StringVar(&metricsAddr, "metrics-addr", "8000", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&enableDebugLog, "enable-debug-log", false,
		"Enable debug mode for controller manager. Enabling this will add debug logs")
	flag.Parse()

	logger := zap.New(zap.UseDevMode(enableDebugLog))
	ctrl.SetLogger(logger)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   electionID,
		Logger:             log.NullLogger{}, // Avoid excessive logs.
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&externalentity.ExternalEntityReconciler{
		Client: mgr.GetClient(),
		Log:    logger.WithName("ExternalEntity"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ExternalEntity")
		os.Exit(1)
	}

	if err = (&webhook.ExternalEntityWebhook{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ExternalEntity")
		os.Exit(1)
	}

	if err = (&federation.ClusterClaimReconciler{
		Client: mgr.GetClient(),
		Log:    logger.WithName("ClusterClaim"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterClaim")
		os.Exit(1)
	}
	if err = (&federation.FedReconciler{
		Client: mgr.GetClient(),
		Log:    logger.WithName("Federation"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Federation")
		os.Exit(1)
	}
	if err = (&federationv1alpha1.ClusterClaim{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterClaim")
		os.Exit(1)
	}
	if err = (&federationv1alpha1.Federation{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Federation")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
