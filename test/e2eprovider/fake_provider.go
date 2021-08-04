package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider/server"
)

var (
	endpoint = flag.String("endpoint", "unix:///tmp/fake-provider.sock", "CSI provier gRPC endpoint")
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	flag.Parse()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)

	fakeProviderServer, err := server.NewSimpleCSIProviderServer(*endpoint)
	if err != nil {
		klog.Fatalf("failed to get new fake provider server, err: %+v", err)
	}

	if err := os.Remove(fakeProviderServer.SocketPath); err != nil && !os.IsNotExist(err) {
		klog.Fatalf("failed to remove %s, error: %s", fakeProviderServer.SocketPath, err.Error())
	}

	err = fakeProviderServer.Start()
	if err != nil {
		klog.Fatalf("failed to start fake provider server, err: %+v", err)
	}

	<-signalChan
	// gracefully stop the grpc server
	klog.Infof("terminating the fake server")
	fakeProviderServer.Stop()
}
