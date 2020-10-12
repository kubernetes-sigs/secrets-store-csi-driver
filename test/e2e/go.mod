module sigs.k8s.io/secrets-store-csi-driver/test/e2e

go 1.13

require (
	github.com/Masterminds/sprig/v3 v3.1.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20200428143746-21a406dcc535 // indirect
	github.com/containerd/containerd v1.3.4 // indirect
	github.com/elazarl/goproxy v0.0.0-20180725130230-947c36da3153 // indirect
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	helm.sh/helm/v3 v3.1.3
	k8s.io/api v0.17.9
	k8s.io/apimachinery v0.17.9
	k8s.io/client-go v0.17.9
	k8s.io/klog v1.0.0
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/cluster-api v0.3.10
	sigs.k8s.io/controller-runtime v0.5.11
	sigs.k8s.io/secrets-store-csi-driver v0.0.14
)

replace sigs.k8s.io/secrets-store-csi-driver => ../..
