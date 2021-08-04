module sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider

go 1.16

require (
	google.golang.org/grpc v1.39.0
	k8s.io/klog/v2 v2.10.0
	sigs.k8s.io/secrets-store-csi-driver v0.0.0-00010101000000-000000000000
	sigs.k8s.io/yaml v1.2.0
)

replace sigs.k8s.io/secrets-store-csi-driver => ../..
