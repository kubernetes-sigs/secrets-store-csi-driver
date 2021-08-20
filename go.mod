module sigs.k8s.io/secrets-store-csi-driver

go 1.16

require (
	github.com/container-storage-interface/spec v1.3.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.5
	github.com/kubernetes-csi/csi-lib-utils v0.7.1
	github.com/kubernetes-csi/csi-test/v4 v4.0.2
	github.com/onsi/gomega v1.13.0
	github.com/prometheus/client_golang v1.11.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/otel v0.20.0
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.20.0
	go.opentelemetry.io/otel/metric v0.20.0
	google.golang.org/grpc v1.39.0
	google.golang.org/protobuf v1.26.0
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/component-base v0.21.1
	k8s.io/klog/v2 v2.10.0
	k8s.io/mount-utils v0.21.0
	sigs.k8s.io/controller-runtime v0.9.0
	sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider v0.0.0-00010101000000-000000000000
)

replace (
	sigs.k8s.io/secrets-store-csi-driver => ./
	sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider => ./test/e2eprovider
)
