module sigs.k8s.io/secrets-store-csi-driver

go 1.16

require (
	github.com/container-storage-interface/spec v1.3.0
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.2
	github.com/kubernetes-csi/csi-lib-utils v0.7.1
	github.com/kubernetes-csi/csi-test/v4 v4.0.2
	github.com/onsi/gomega v1.10.2
	github.com/prometheus/client_golang v1.8.0
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/otel v0.13.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.13.0
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	google.golang.org/grpc v1.29.1
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/component-base v0.20.2
	k8s.io/klog/v2 v2.8.0
	k8s.io/mount-utils v0.21.0
	sigs.k8s.io/controller-runtime v0.8.2
)
