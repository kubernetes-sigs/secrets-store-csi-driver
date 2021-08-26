module sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider

go 1.17

replace sigs.k8s.io/secrets-store-csi-driver => ../..

require (
	github.com/google/go-cmp v0.5.6
	google.golang.org/grpc v1.40.0
	k8s.io/klog/v2 v2.10.0
	sigs.k8s.io/secrets-store-csi-driver v0.0.0-00010101000000-000000000000
	sigs.k8s.io/yaml v1.2.0
)

require (
	github.com/go-logr/logr v0.4.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023 // indirect
	golang.org/x/sys v0.0.0-20210616094352-59db8d763f22 // indirect
	golang.org/x/text v0.3.6 // indirect
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c // indirect
	google.golang.org/protobuf v1.26.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
