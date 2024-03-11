/*
Copyright 2024 The Kubernetes Authors.

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
	"time"

	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	secretsstorecsiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	secretsyncv1alpha1 "sigs.k8s.io/secrets-store-csi-driver/secret-sync-controller/api/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/secret-sync-controller/controllers"
	"sigs.k8s.io/secrets-store-csi-driver/secret-sync-controller/pkg/k8s"
	"sigs.k8s.io/secrets-store-csi-driver/secret-sync-controller/pkg/provider"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(secretsyncv1alpha1.AddToScheme(scheme))

	utilruntime.Must(secretsstorecsiv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var tokenRequestAudiences string
	var providerVolumePath string
	var maxCallRecvMsgSize int

	var rotationPollInterval *time.Duration = flag.Duration("rotation-poll-interval", time.Duration(21600)*time.Second, "Secret rotation poll interval duration")

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8085", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&providerVolumePath, "provider-volume", "/provider", "Volume path for provider.")
	flag.IntVar(&maxCallRecvMsgSize, "max-call-recv-msg-size", 1024*1024*4, "maximum size in bytes of gRPC response from plugins")
	flag.StringVar(&tokenRequestAudiences, "token-request-audience", "", "Audience for the token request, comma separated.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "29f1d54e.secret-sync.x-k8s.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// token request client
	kubeClient := kubernetes.NewForConfigOrDie(ctrl.GetConfigOrDie())
	tokenClient := k8s.NewTokenClient(kubeClient)
	if err != nil {
		setupLog.Error(err, "failed to create token client")
		os.Exit(1)
	}

	providerClients := provider.NewPluginClientBuilder([]string{providerVolumePath}, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxCallRecvMsgSize)))
	defer providerClients.Cleanup()

	audiences := strings.Split(tokenRequestAudiences, ",")
	if len(tokenRequestAudiences) == 0 {
		audiences = []string{}
	}

	if err = (&controllers.SecretSyncReconciler{
		Clientset:            kubeClient,
		Client:               mgr.GetClient(),
		Scheme:               mgr.GetScheme(),
		TokenClient:          tokenClient,
		ProviderClients:      providerClients,
		Audiences:            audiences,
		RotationPollInterval: *rotationPollInterval,
		EventRecorder:        record.NewBroadcaster().NewRecorder(scheme, corev1.EventSource{Component: "secret-sync-controller"}),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SecretSync")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
