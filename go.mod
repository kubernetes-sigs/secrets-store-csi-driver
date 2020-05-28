module sigs.k8s.io/secrets-store-csi-driver

go 1.12

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/container-storage-interface/spec v1.0.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/golang/protobuf v1.3.4
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/kubernetes-csi/csi-test v1.1.0
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/testify v1.4.0
	go.opentelemetry.io/otel v0.4.3
	go.opentelemetry.io/otel/exporters/metric/prometheus v0.4.3
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/zap v1.10.0 // indirect
	golang.org/x/crypto v0.0.0-20200414173820-0848c9571904 // indirect
	golang.org/x/net v0.0.0-20190923162816-aa69164e4478
	golang.org/x/oauth2 v0.0.0-20190402181905-9f3314589c9a // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	google.golang.org/grpc v1.27.1
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/klog v0.4.0 // indirect
	k8s.io/utils v0.0.0-20200229041039-0a110f9eb7ab
	sigs.k8s.io/controller-runtime v0.2.0
)
