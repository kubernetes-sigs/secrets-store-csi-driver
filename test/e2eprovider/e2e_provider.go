//go:build e2e
// +build e2e

/*
Copyright 2021 The Kubernetes Authors.

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
	"os"
	"os/signal"
	"syscall"
	"time"

	"sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider/server"

	"k8s.io/klog/v2"
	"monis.app/mlog"
)

var (
	endpoint = flag.String("endpoint", "unix:///tmp/e2e-provider.sock", "CSI provier gRPC endpoint")
)

func main() {
	if err := mainErr(); err != nil {
		mlog.Fatal(err)
	}
}

func mainErr() error {
	klog.InitFlags(nil)
	defer klog.Flush()

	flag.Parse()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)

	mockProviderServer, err := server.NewE2EProviderServer(*endpoint)
	if err != nil {
		klog.ErrorS(err, "failed to get new mock e2e provider server")
		return err
	}

	if err := os.Remove(mockProviderServer.GetSocketPath()); err != nil && !os.IsNotExist(err) {
		klog.ErrorS(err, "failed to remove unix domain socket", "socketPath", mockProviderServer.GetSocketPath())
		return err
	}

	err = mockProviderServer.Start()
	if err != nil {
		klog.ErrorS(err, "failed to start mock e2e provider server")
		return err
	}

	// endpoint to enable rotation response.
	// rotation response ("rotated") might be triggered by rotation reconciler before other tests. This results in failure of those tests.
	// To avoid this, we enable rotation response only when we are ready to run rotation tests.
	go func() {
		// set ROTATION_ENABLED=false to disable rotation response logic.
		os.Setenv("ROTATION_ENABLED", "false")

		http.HandleFunc("/rotation", server.RotationHandler)
		http.HandleFunc("/validate-token-requests", server.ValidateTokenAudienceHandler)

		server := &http.Server{
			Addr:              ":8080",
			ReadHeaderTimeout: 5 * time.Second,
		}
		klog.Fatal(server.ListenAndServe())
	}()

	<-signalChan
	// gracefully stop the grpc server
	klog.InfoS("terminating the e2e provider server")
	mockProviderServer.Stop()
	return nil
}
