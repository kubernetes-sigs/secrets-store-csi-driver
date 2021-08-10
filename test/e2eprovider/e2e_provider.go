package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider/server"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider/vault"

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

	// Get fake Vault
	vault, err := vault.NewVault()
	if err != nil {
		klog.Fatalf("failed to get new fake vault, err: %+v", err)
	}

	fakeProviderServer, err := server.NewE2eProviderServer(*endpoint, vault)
	if err != nil {
		klog.Fatalf("failed to get new fake e2e provider server, err: %+v", err)
	}

	if err := os.Remove(fakeProviderServer.SocketPath); err != nil && !os.IsNotExist(err) {
		klog.Fatalf("failed to remove %s, error: %s", fakeProviderServer.SocketPath, err.Error())
	}

	err = fakeProviderServer.Start()
	if err != nil {
		klog.Fatalf("failed to start fake e2e provider server, err: %+v", err)
	}

	<-signalChan
	// gracefully stop the grpc server
	klog.Infof("terminating the e2e provider server")
	fakeProviderServer.Stop()
}
