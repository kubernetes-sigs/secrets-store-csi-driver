/*
Copyright 2018 The Kubernetes Authors.

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
	"time"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/metrics"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/rotation"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	json "k8s.io/component-base/logs/json"
	"k8s.io/klog/v2"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/controllers"
	secretsstore "sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store"
	// +kubebuilder:scaffold:imports
)

var (
	endpoint           = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	driverName         = flag.String("drivername", "secrets-store.csi.k8s.io", "name of the driver")
	nodeID             = flag.String("nodeid", "", "node id")
	debug              = flag.Bool("debug", false, "sets log to debug level [DEPRECATED]. Use -v=<log level> to configure log level.")
	logFormatJSON      = flag.Bool("log-format-json", false, "set log formatter to json")
	providerVolumePath = flag.String("provider-volume", "/etc/kubernetes/secrets-store-csi-providers", "Volume path for provider")
	minProviderVersion = flag.String("min-provider-version", "", "set minimum supported provider versions with current driver")
	metricsAddr        = flag.String("metrics-addr", ":8095", "The address the metric endpoint binds to")
	// grpcSupportedProviders is a ; separated string that can contain a list of providers. The reason it's a string is to allow scenarios
	// where the driver is being used with 2 providers, one which supports grpc and other using binary for provider.
	grpcSupportedProviders = flag.String("grpc-supported-providers", "", "set list of providers that support grpc for driver-provider [alpha]")
	enableSecretRotation   = flag.Bool("enable-secret-rotation", false, "Enable secret rotation feature [alpha]")
	rotationPollInterval   = flag.Duration("rotation-poll-interval", 2*time.Minute, "Secret rotation poll interval duration")

	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	flag.Parse()

	if *logFormatJSON {
		klog.SetLogger(json.JSONLogger)
	}
	if *debug {
		klog.Warning("--debug flag has been DEPRECATED and will be removed in future releases. Use -v=<log level> to configure log verbosity.")
	}

	// initialize metrics exporter before creating measurements
	// Issue: https://github.com/open-telemetry/opentelemetry-go/issues/677
	// this has been resolved in otel release v0.5.0
	// TODO (aramase) update to latest version of otel and deps
	m, err := metrics.NewMetricsExporter()
	if err != nil {
		klog.Fatalf("failed to initialize metrics exporter, error: %+v", err)
	}
	defer m.Stop()

	config := ctrl.GetConfigOrDie()
	config.UserAgent = "csi-secrets-store/controller"

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: *metricsAddr,
		LeaderElection:     false,
	})
	if err != nil {
		klog.Fatalf("failed to start manager, error: %+v", err)
	}

	reconciler, err := controllers.New(mgr, *nodeID)
	if err != nil {
		klog.Fatalf("failed to create secret provider class pod status reconciler, error: %+v", err)
	}
	if err = reconciler.SetupWithManager(mgr); err != nil {
		klog.Fatalf("failed to create controller, error: %+v", err)
	}
	// +kubebuilder:scaffold:builder

	stopCh := ctrl.SetupSignalHandler()
	go func() {
		klog.Infof("starting manager")
		if err := mgr.Start(stopCh); err != nil {
			klog.Fatalf("failed to run manager, error: %+v", err)
		}
	}()

	go func() {
		reconciler.RunPatcher(stopCh)
	}()

	if *enableSecretRotation {
		rec, err := rotation.NewReconciler(scheme, *providerVolumePath, *nodeID, *rotationPollInterval)
		if err != nil {
			klog.Fatalf("failed to initialize rotation reconciler, error: %+v", err)
		}
		stopCh := make(<-chan struct{})
		go rec.Run(stopCh)
	}

	handle()
}

func handle() {
	driver := secretsstore.GetDriver()
	cfg, err := config.GetConfig()
	if err != nil {
		klog.Fatalf("failed to initialize driver, error getting config: %+v", err)
	}
	c, err := client.New(cfg, client.Options{Scheme: scheme, Mapper: nil})
	if err != nil {
		klog.Fatalf("failed to initialize driver, error creating client: %+v", err)
	}
	driver.Run(*driverName, *nodeID, *endpoint, *providerVolumePath, *minProviderVersion, *grpcSupportedProviders, c)
}
