//go:build e2e
// +build e2e

package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider/server"

	"k8s.io/klog/v2"
)

var (
	endpoint = flag.String("endpoint", "unix:///tmp/e2e-provider.sock", "CSI provier gRPC endpoint")
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	flag.Parse()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)

	mockProviderServer, err := server.NewE2EProviderServer(*endpoint)
	if err != nil {
		klog.Fatalf("failed to get new mock e2e provider server, err: %+v", err)
	}

	if err := os.Remove(mockProviderServer.GetSocketPath()); err != nil && !os.IsNotExist(err) {
		klog.Fatalf("failed to remove %s, error: %s", mockProviderServer.GetSocketPath(), err.Error())
	}

	err = mockProviderServer.Start()
	if err != nil {
		klog.Fatalf("failed to start mock e2e provider server, err: %+v", err)
	}

	// endpoint to enable rotation response.
	// rotation response ("rotated") might be triggered by rotation reconciler before other tests. This results in failure of those tests.
	// To avoid this, we enable rotation response only when we are ready to run rotation tests.
	go func() {
		// set ROTATION_ENABLED=false to disable rotation response logic.
		os.Setenv("ROTATION_ENABLED", "false")

		http.HandleFunc("/rotation", server.RotationHandler)
		klog.Fatal(http.ListenAndServe(":8080", nil))
	}()

	<-signalChan
	// gracefully stop the grpc server
	klog.InfoS("terminating the e2e provider server")
	mockProviderServer.Stop()
}
