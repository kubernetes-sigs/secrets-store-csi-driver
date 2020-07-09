/*
Copyright 2020 The Kubernetes Authors.

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
	"io/ioutil"
	"os"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"k8s.io/utils/mount"
)

func testNodeServer(mountPoints []mount.MountPoint) (*nodeServer, error) {
	return newNodeServer(NewFakeDriver(), "", "", "testnode", mount.NewFakeMounter(mountPoints))
}

func getTestTargetPath(t *testing.T) string {
	dir, err := ioutil.TempDir("", "ut")
	if err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}
	return dir
}

func TestNodePublishVolume(t *testing.T) {
	tests := []struct {
		name              string
		nodePublishVolReq csi.NodePublishVolumeRequest
		mountPoints       []mount.MountPoint
		expectedErr       bool
	}{
		{
			name:              "volume capabilities nil",
			nodePublishVolReq: csi.NodePublishVolumeRequest{},
			expectedErr:       true,
		},
		{
			name: "volume id is empty",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
			},
			expectedErr: true,
		},
		{
			name: "target path is empty",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
			},
			expectedErr: true,
		},
		{
			name: "volume context is not set",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       getTestTargetPath(t),
			},
			expectedErr: true,
		},
		{
			name: "secret provider class not found",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       getTestTargetPath(t),
				VolumeContext:    map[string]string{"secretProviderClass": "provider1"},
			},
			expectedErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.nodePublishVolReq.TargetPath != "" {
				defer os.RemoveAll(test.nodePublishVolReq.TargetPath)
			}
			ns, err := testNodeServer(test.mountPoints)
			if err != nil {
				t.Fatalf("expected error to be nil, got: %+v", err)
			}

			_, err = ns.NodePublishVolume(context.TODO(), &test.nodePublishVolReq)
			if test.expectedErr && err == nil || !test.expectedErr && err != nil {
				t.Fatalf("expected err: %v, got: %+v", test.expectedErr, err)
			}
		})
	}
}
