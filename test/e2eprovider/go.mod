module sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider

go 1.19

replace sigs.k8s.io/secrets-store-csi-driver => ../..

require (
	github.com/google/go-cmp v0.5.8
	google.golang.org/grpc v1.47.0
	k8s.io/klog/v2 v2.80.1
	sigs.k8s.io/secrets-store-csi-driver v0.0.0-00010101000000-000000000000
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	golang.org/x/net v0.4.0 // indirect
	golang.org/x/sys v0.3.0 // indirect
	golang.org/x/text v0.5.0 // indirect
	google.golang.org/genproto v0.0.0-20220502173005-c8bf987b8c21 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
