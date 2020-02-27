/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package secretsstore

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	csicommon "sigs.k8s.io/secrets-store-csi-driver/pkg/csi-common"

	wrappers "github.com/golang/protobuf/ptypes/wrappers"
)

type identityServer struct {
	*csicommon.DefaultIdentityServer
}

// Probe check whether the plugin is running or not.
// Currently the spec does not dictate what you should return.
// Returning ready=true as ability to connect to the driver and make Probe RPC call
// means driver is working as expected.
func (ids *identityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{Ready: &wrappers.BoolValue{Value: true}}, nil
}
