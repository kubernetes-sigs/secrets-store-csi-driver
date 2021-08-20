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
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestProbe(t *testing.T) {
	ids := newIdentityServer("secrets-store.csi.k8s.io", "v1.0.0")

	resp, err := ids.Probe(context.Background(), &csi.ProbeRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.GetReady().GetValue())
}

func TestGetPluginInfo(t *testing.T) {
	tests := []struct {
		name string
		ids  *identityServer
		want *csi.GetPluginInfoResponse
		err  error
	}{
		{
			name: "success",
			ids:  newIdentityServer("secrets-store.csi.k8s.io", "v1.0.0"),
			want: &csi.GetPluginInfoResponse{
				Name:          "secrets-store.csi.k8s.io",
				VendorVersion: "v1.0.0",
			},
			err: nil,
		},
		{
			name: "driver name not configured",
			ids:  newIdentityServer("", "v1.0.0"),
			want: nil,
			err:  status.Error(codes.Unavailable, "driver name not configured"),
		},
		{
			name: "driver version not configured",
			ids:  newIdentityServer("secrets-store.csi.k8s.io", ""),
			want: nil,
			err:  status.Error(codes.Unavailable, "driver version not configured"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.ids.GetPluginInfo(context.Background(), &csi.GetPluginInfoRequest{})
			assert.Equal(t, test.err, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestGetPluginCapabilities(t *testing.T) {
	ids := newIdentityServer("secrets-store.csi.k8s.io", "v1.0.0")

	resp, err := ids.GetPluginCapabilities(context.Background(), &csi.GetPluginCapabilitiesRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, resp.GetCapabilities()[0].Type, &csi.PluginCapability_Service_{
		Service: &csi.PluginCapability_Service{
			Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
		},
	})
}
