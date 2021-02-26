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
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof" // #nosec
	"time"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/cache"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/metrics"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/rotation"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/version"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	json "k8s.io/component-base/logs/json"
	"k8s.io/klog/v2"

	ctrl "sigs.k8s.io/controller-runtime"

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
	// this will be removed in a future release
	metricsAddr          = flag.String("metrics-addr", ":8095", "The address the metric endpoint binds to")
	_                    = flag.String("grpc-supported-providers", "", "[DEPRECATED] set list of providers that support grpc for driver-provider [alpha]")
	enableSecretRotation = flag.Bool("enable-secret-rotation", false, "Enable secret rotation feature [alpha]")
	rotationPollInterval = flag.Duration("rotation-poll-interval", 2*time.Minute, "Secret rotation poll interval duration")
	enableProfile        = flag.Bool("enable-pprof", false, "enable pprof profiling")
	profilePort          = flag.Int("pprof-port", 6065, "port for pprof profiling")

	// enable filtered watch for NodePublishSecretRef secrets. The filtering is done on the csi driver label: secrets-store.csi.k8s.io/used=true
	// For Kubernetes secrets used to provide credentials for use with the CSI driver, set the label by running: kubectl label secret secrets-store-creds secrets-store.csi.k8s.io/used=true
	// This feature flag will be enabled by default after n+2 releases giving time for users to label all their existing credential secrets.
	filteredWatchSecret = flag.Bool("filtered-watch-secret", false, "enable filtered watch for NodePublishSecretRef secrets with label secrets-store.csi.k8s.io/used=true")

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
	if *enableProfile {
		klog.Infof("Starting profiling on port %d", *profilePort)
		go func() {
			addr := fmt.Sprintf("%s:%d", "localhost", *profilePort)
			klog.ErrorS(http.ListenAndServe(addr, nil), "unable to start profiling server")
		}()
	}
	if *filteredWatchSecret {
		klog.Infof("Filtered watch for nodePublishSecretRef secret based on secrets-store.csi.k8s.io/used=true label enabled")
	}

	// initialize metrics exporter before creating measurements
	err := metrics.InitMetricsExporter()
	if err != nil {
		klog.Fatalf("failed to initialize metrics exporter, error: %+v", err)
	}

	cfg := ctrl.GetConfigOrDie()
	cfg.UserAgent = version.GetUserAgent("controller")

	// this enables filtered watch of pods based on the node name
	// only pods running on the same node as the csi driver will be cached
	fieldSelectorByResource := map[schema.GroupResource]string{
		{Group: "", Resource: "pods"}: fields.OneTermEqualSelector("spec.nodeName", *nodeID).String(),
	}
	labelSelectorByResource := map[schema.GroupResource]string{
		// this enables filtered watch of secretproviderclasspodstatuses based on the internal node label
		// internal.secrets-store.csi.k8s.io/node-name=<node name> added by csi driver
		{Group: v1alpha1.GroupVersion.Group, Resource: "secretproviderclasspodstatuses"}: fmt.Sprintf("%s=%s", v1alpha1.InternalNodeLabel, *nodeID),
		// this enables filtered watch of secrets based on the label (secrets-store.csi.k8s.io/managed=true)
		// added to the secrets created by the CSI driver
		{Group: "", Resource: "secrets"}: fmt.Sprintf("%s=true", controllers.SecretManagedLabel),
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: *metricsAddr,
		LeaderElection:     false,
		NewCache: cache.Builder(cache.Options{
			FieldSelectorByResource: fieldSelectorByResource,
			LabelSelectorByResource: labelSelectorByResource,
		}),
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

	ctx := withShutdownSignal(context.Background())

	// create provider clients
	providerClients := secretsstore.NewPluginClientBuilder(*providerVolumePath)
	defer providerClients.Cleanup()

	go func() {
		klog.Infof("starting manager")
		if err := mgr.Start(ctx); err != nil {
			klog.Fatalf("failed to run manager, error: %+v", err)
		}
	}()

	go func() {
		reconciler.RunPatcher(ctx)
	}()

	if *enableSecretRotation {
		rec, err := rotation.NewReconciler(scheme, *providerVolumePath, *nodeID, *rotationPollInterval, providerClients, *filteredWatchSecret)
		if err != nil {
			klog.Fatalf("failed to initialize rotation reconciler, error: %+v", err)
		}
		go rec.Run(ctx.Done())
	}

	driver := secretsstore.GetDriver()
	driver.Run(ctx, *driverName, *nodeID, *endpoint, *providerVolumePath, providerClients, mgr.GetClient())
}

// withShutdownSignal returns a copy of the parent context that will close if
// the process receives termination signals.
func withShutdownSignal(ctx context.Context) context.Context {
	nctx, cancel := context.WithCancel(ctx)
	stopCh := ctrl.SetupSignalHandler().Done()

	go func() {
		<-stopCh
		klog.Info("received shutdown signal")
		cancel()
	}()
	return nctx
}
