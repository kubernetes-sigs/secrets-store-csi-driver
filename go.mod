module sigs.k8s.io/secrets-store-csi-driver

go 1.15

require (
	github.com/blang/semver v3.5.0+incompatible
	github.com/container-storage-interface/spec v1.3.0
	github.com/go-logr/logr v0.2.1 // indirect
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.2
	github.com/kubernetes-csi/csi-lib-utils v0.7.1
	github.com/kubernetes-csi/csi-test/v4 v4.0.2
	github.com/onsi/gomega v1.10.1
	github.com/prometheus/client_golang v1.8.0 // indirect
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/otel v0.13.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.13.0
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	golang.org/x/sys v0.0.0-20201107080550-4d91cf3a1aaf // indirect
	google.golang.org/grpc v1.29.1
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v0.19.3
	k8s.io/component-base v0.19.3
	k8s.io/klog/v2 v2.3.0
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
	sigs.k8s.io/controller-runtime v0.6.3
)
