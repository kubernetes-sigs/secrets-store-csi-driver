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
	"path/filepath"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/mount"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
)

func testNodeServer(mountPoints []mount.MountPoint, client client.Client) (*nodeServer, error) {
	tmpDir, err := ioutil.TempDir("", "ut")
	if err != nil {
		return nil, err
	}
	return newNodeServer(NewFakeDriver(), tmpDir, "", "testnode", mount.NewFakeMounter(mountPoints), client)
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
		initObjects       []runtime.Object
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
		{
			name: "secret provider class in pod namespace not found",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       getTestTargetPath(t),
				VolumeContext:    map[string]string{"secretProviderClass": "provider1", csipodname: "pod1", csipodnamespace: "default"},
			},
			initObjects: []runtime.Object{
				&v1alpha1.SecretProviderClass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "provider1",
						Namespace: "testns",
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "provider not set in secret provider class",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       getTestTargetPath(t),
				VolumeContext:    map[string]string{"secretProviderClass": "provider1", csipodname: "pod1", csipodnamespace: "default"},
			},
			initObjects: []runtime.Object{
				&v1alpha1.SecretProviderClass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "provider1",
						Namespace: "default",
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "parameters not set in secret provider class",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       getTestTargetPath(t),
				VolumeContext:    map[string]string{"secretProviderClass": "provider1", csipodname: "pod1", csipodnamespace: "default"},
			},
			initObjects: []runtime.Object{
				&v1alpha1.SecretProviderClass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "provider1",
						Namespace: "default",
					},
					Spec: v1alpha1.SecretProviderClassSpec{
						Provider: "provider1",
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "read only is not set to true",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       getTestTargetPath(t),
				VolumeContext:    map[string]string{"secretProviderClass": "provider1", csipodname: "pod1", csipodnamespace: "default"},
			},
			initObjects: []runtime.Object{
				&v1alpha1.SecretProviderClass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "provider1",
						Namespace: "default",
					},
					Spec: v1alpha1.SecretProviderClassSpec{
						Provider:   "provider1",
						Parameters: map[string]string{"parameter1": "value1"},
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "failed to invoke provider, unmounted to force retry",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       getTestTargetPath(t),
				VolumeContext:    map[string]string{"secretProviderClass": "provider1", csipodname: "pod1", csipodnamespace: "default", csipoduid: "poduid1"},
				Readonly:         true,
			},
			initObjects: []runtime.Object{
				&v1alpha1.SecretProviderClass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "provider1",
						Namespace: "default",
					},
					Spec: v1alpha1.SecretProviderClassSpec{
						Provider:   "provider1",
						Parameters: map[string]string{"parameter1": "value1"},
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "volume already mounted, no remount",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       getTestTargetPath(t),
				VolumeContext:    map[string]string{"secretProviderClass": "provider1", csipodname: "pod1", csipodnamespace: "default", csipoduid: "poduid1"},
				Readonly:         true,
			},
			initObjects: []runtime.Object{
				&v1alpha1.SecretProviderClass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "provider1",
						Namespace: "default",
					},
					Spec: v1alpha1.SecretProviderClassSpec{
						Provider:   "provider1",
						Parameters: map[string]string{"parameter1": "value1"},
					},
				},
			},
			mountPoints: []mount.MountPoint{},
			expectedErr: false,
		},
	}

	s := scheme.Scheme
	s.AddKnownTypes(v1alpha1.GroupVersion,
		&v1alpha1.SecretProviderClass{},
		&v1alpha1.SecretProviderClassList{},
	)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.nodePublishVolReq.TargetPath != "" {
				defer os.RemoveAll(test.nodePublishVolReq.TargetPath)
			}
			if test.mountPoints != nil {
				// If file is a symlink, get its absolute path
				absFile, err := filepath.EvalSymlinks(test.nodePublishVolReq.TargetPath)
				if err != nil {
					absFile = test.nodePublishVolReq.TargetPath
				}
				test.mountPoints = append(test.mountPoints, mount.MountPoint{Path: absFile})
			}
			ns, err := testNodeServer(test.mountPoints, fake.NewFakeClientWithScheme(s, test.initObjects...))
			if err != nil {
				t.Fatalf("expected error to be nil, got: %+v", err)
			}

			_, err = ns.NodePublishVolume(context.TODO(), &test.nodePublishVolReq)
			if test.expectedErr && err == nil || !test.expectedErr && err != nil {
				t.Fatalf("expected err: %v, got: %+v", test.expectedErr, err)
			}
			mnts, err := ns.mounter.List()
			if err != nil {
				t.Fatalf("expected err to be nil, got: %v", err)
			}
			if test.expectedErr && len(test.mountPoints) == 0 && len(mnts) != 0 {
				t.Fatalf("expected mount points to be 0")
			}
		})
	}
}
