/*
Copyright 2024 DevOps Click.

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

// Package main is the entrypoint for the NGINX Operator.
// It initializes the controller manager with leader election, health probes,
// metrics, and registers all controllers (NginxServer, NginxRoute, NginxUpstream).
//
// Usage:
//
//	./operator [flags]
//	./operator --debug
//	./operator --leader-elect=true --metrics-bind-address=:8080
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	nginxv1alpha1 "github.com/devops-click/nginx-operator/api/v1alpha1"
	"github.com/devops-click/nginx-operator/internal/config"
	"github.com/devops-click/nginx-operator/internal/controller"
	"github.com/devops-click/nginx-operator/internal/nginx"
	"github.com/devops-click/nginx-operator/internal/version"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(nginxv1alpha1.AddToScheme(scheme))
}

func main() {
	// --- CLI Flags ---
	var (
		metricsAddr          string
		probeAddr            string
		enableLeaderElection bool
		leaderElectionID     string
		leaderElectionNS     string
		leaseDuration        time.Duration
		renewDeadline        time.Duration
		retryPeriod          time.Duration
		debug                bool
		showVersion          bool
		reloaderTag          string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&leaderElectionID, "leader-election-id", "nginx-operator-leader.nginx.devops.click",
		"The name of the resource used for leader election.")
	flag.StringVar(&leaderElectionNS, "leader-election-namespace", "",
		"The namespace for leader election resource. Defaults to the operator's namespace.")
	flag.DurationVar(&leaseDuration, "lease-duration", 15*time.Second,
		"Duration that non-leader candidates will wait before forcing leader acquisition.")
	flag.DurationVar(&renewDeadline, "renew-deadline", 10*time.Second,
		"Duration the acting leader will retry refreshing leadership before giving up.")
	flag.DurationVar(&retryPeriod, "retry-period", 2*time.Second,
		"Duration between leader election retries.")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging (verbose mode).")
	flag.BoolVar(&showVersion, "version", false, "Print version information and exit.")
	flag.StringVar(&reloaderTag, "reloader-tag", "", "Override the config reloader image tag. Defaults to operator version.")
	flag.Parse()

	// --- Version ---
	if showVersion {
		fmt.Println(version.Get().String())
		os.Exit(0)
	}

	// --- Logger ---
	logLevel := zapcore.InfoLevel
	if debug {
		logLevel = zapcore.DebugLevel
	}
	opts := zap.Options{
		Development: debug,
		Level:       logLevel,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	versionInfo := version.Get()
	setupLog.Info("starting NGINX Operator",
		"version", versionInfo.Version,
		"commit", versionInfo.GitCommit,
		"buildDate", versionInfo.BuildDate,
		"go", versionInfo.GoVersion,
		"platform", versionInfo.Platform,
	)

	// --- Controller Manager ---
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        leaderElectionID,
		LeaderElectionNamespace: leaderElectionNS,
		LeaseDuration:           &leaseDuration,
		RenewDeadline:           &renewDeadline,
		RetryPeriod:             &retryPeriod,
	})
	if err != nil {
		setupLog.Error(err, "unable to create controller manager")
		os.Exit(1)
	}

	// --- Config Generator ---
	configGen, err := config.NewGenerator()
	if err != nil {
		setupLog.Error(err, "unable to create config generator")
		os.Exit(1)
	}

	// --- Resource Manager ---
	resourceMgr := nginx.NewResourceManager(mgr.GetClient(), mgr.GetScheme())

	// --- Set reloader tag ---
	if reloaderTag == "" {
		reloaderTag = versionInfo.Version
	}

	// --- Register Controllers ---

	// NginxServer Controller
	if err := (&controller.NginxServerReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		ConfigGen:   configGen,
		ResourceMgr: resourceMgr,
		ReloaderTag: reloaderTag,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NginxServer")
		os.Exit(1)
	}

	// NginxRoute Controller
	if err := (&controller.NginxRouteReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		ConfigGen: configGen,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NginxRoute")
		os.Exit(1)
	}

	// NginxUpstream Controller
	if err := (&controller.NginxUpstreamReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		ConfigGen: configGen,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NginxUpstream")
		os.Exit(1)
	}

	// --- Health Probes ---
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// --- Start ---
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
