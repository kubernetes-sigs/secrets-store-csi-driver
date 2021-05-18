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
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store/mocks"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/test_utils/tmpdir"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	mount "k8s.io/mount-utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testNodeServer(t *testing.T, mountPoints []mount.MountPoint, client client.Client, reporter StatsReporter) (*nodeServer, error) {
	t.Helper()
	tmpDir := tmpdir.New(t, "", "ut")
	providerClients := NewPluginClientBuilder(tmpDir)
	return newNodeServer(NewFakeDriver(), tmpDir, "testnode", mount.NewFakeMounter(mountPoints), providerClients, client, reporter)
}

func TestNodePublishVolume(t *testing.T) {
	tests := []struct {
		name               string
		nodePublishVolReq  csi.NodePublishVolumeRequest
		mountPoints        []mount.MountPoint
		initObjects        []runtime.Object
		RPCCode            codes.Code
		wantsRPCCode       bool
		expectedErr        bool
		shouldRetryRemount bool
	}{
		{
			name:               "volume capabilities nil",
			nodePublishVolReq:  csi.NodePublishVolumeRequest{},
			RPCCode:            codes.InvalidArgument,
			wantsRPCCode:       true,
			expectedErr:        true,
			shouldRetryRemount: true,
		},
		{
			name: "volume id is empty",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
			},
			RPCCode:            codes.InvalidArgument,
			wantsRPCCode:       true,
			expectedErr:        true,
			shouldRetryRemount: true,
		},
		{
			name: "target path is empty",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
			},
			RPCCode:            codes.InvalidArgument,
			wantsRPCCode:       true,
			expectedErr:        true,
			shouldRetryRemount: true,
		},
		{
			name: "volume context is not set",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       tmpdir.New(t, "", "ut"),
			},
			RPCCode:            codes.InvalidArgument,
			wantsRPCCode:       true,
			expectedErr:        true,
			shouldRetryRemount: true,
		},
		{
			name: "secret provider class not found",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       tmpdir.New(t, "", "ut"),
				VolumeContext:    map[string]string{"secretProviderClass": "provider1"},
			},
			expectedErr:        true,
			shouldRetryRemount: true,
		},
		{
			name: "secret provider class in pod namespace not found",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       tmpdir.New(t, "", "ut"),
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
			expectedErr:        true,
			shouldRetryRemount: true,
		},
		{
			name: "provider not set in secret provider class",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       tmpdir.New(t, "", "ut"),
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
			expectedErr:        true,
			shouldRetryRemount: true,
		},
		{
			name: "parameters not set in secret provider class",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       tmpdir.New(t, "", "ut"),
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
			expectedErr:        true,
			shouldRetryRemount: true,
		},
		{
			name: "read only is not set to true",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       tmpdir.New(t, "", "ut"),
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
			expectedErr:        true,
			shouldRetryRemount: true,
		},
		{
			name: "volume already mounted, no remount",
			nodePublishVolReq: csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       tmpdir.New(t, "", "ut"),
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
			mountPoints:        []mount.MountPoint{},
			expectedErr:        false,
			shouldRetryRemount: true,
		},
	}

	s := scheme.Scheme
	s.AddKnownTypes(schema.GroupVersion{Group: v1alpha1.GroupVersion.Group, Version: v1alpha1.GroupVersion.Version},
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
			r := mocks.NewFakeReporter()
			ns, err := testNodeServer(t, test.mountPoints, fake.NewFakeClientWithScheme(s, test.initObjects...), r)
			if err != nil {
				t.Fatalf("expected error to be nil, got: %+v", err)
			}

			numberOfAttempts := 1
			// to ensure the remount is tried after previous failure and still fails
			if test.shouldRetryRemount {
				numberOfAttempts = 2
			}

			for numberOfAttempts > 0 {
				// How many times 'total_node_publish' and 'total_node_publish_error' counters have been incremented so far
				c, cErr := r.ReportNodePublishCtMetricInvoked(), r.ReportNodePublishErrorCtMetricInvoked()

				_, err = ns.NodePublishVolume(context.TODO(), &test.nodePublishVolReq)
				if test.expectedErr && err == nil || !test.expectedErr && err != nil {
					t.Fatalf("expected err: %v, got: %+v", test.expectedErr, err)
				}

				// For the error cases where it is expected that the error will contain an RPC code
				gRPCStatus, ok := status.FromError(err)
				if test.wantsRPCCode && (!ok || test.RPCCode != gRPCStatus.Code()) {
					t.Fatalf("expected RPC status code: %v, got: %v\n", test.RPCCode, gRPCStatus.Code())
				}

				// Check that the correct counter has been incremented
				if err != nil && r.ReportNodePublishErrorCtMetricInvoked() <= cErr {
					t.Fatalf("expected 'total_node_publish_error' counter to be incremented, but it was not")
				}
				if err == nil && r.ReportNodePublishCtMetricInvoked() <= c {
					t.Fatalf("expected 'total_node_publish' counter to be incremented, but it was not")
				}

				mnts, err := ns.mounter.List()
				if err != nil {
					t.Fatalf("expected err to be nil, got: %v", err)
				}
				if test.expectedErr && len(test.mountPoints) == 0 && len(mnts) != 0 {
					t.Fatalf("expected mount points to be 0")
				}
				numberOfAttempts--
			}
		})
	}
}

func TestMountSecretsStoreObjectContent(t *testing.T) {
	tests := []struct {
		name                string
		attributes          string
		secrets             string
		targetPath          string
		permission          string
		expectedErrorReason string
		expectedErr         bool
	}{
		{
			name:        "attributes empty",
			expectedErr: true,
		},
		{
			name:        "target path empty",
			attributes:  "{}",
			expectedErr: true,
		},
		{
			name:        "permission not set",
			attributes:  "{}",
			targetPath:  tmpdir.New(t, "", "ut"),
			expectedErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ns, err := testNodeServer(t, nil, fake.NewFakeClientWithScheme(nil), mocks.NewFakeReporter())
			if err != nil {
				t.Fatalf("expected error to be nil, got: %+v", err)
			}
			_, errorReason, err := ns.mountSecretsStoreObjectContent(context.TODO(), "provider1", test.attributes, test.secrets, test.targetPath, test.permission, "pod")
			if errorReason != test.expectedErrorReason {
				t.Fatalf("expected error reason to be %s, got: %s", test.expectedErrorReason, errorReason)
			}
			if test.expectedErr && err == nil || !test.expectedErr && err != nil {
				t.Fatalf("expected err: %v, got: %+v", test.expectedErr, err)
			}
		})
	}
}

func TestNodeUnpublishVolume(t *testing.T) {
	tests := []struct {
		name                string
		nodeUnpublishVolReq csi.NodeUnpublishVolumeRequest
		mountPoints         []mount.MountPoint
		RPCCode             codes.Code
		wantsErr            bool
		wantsRPCCode        bool
		shouldRetryUnmount  bool
	}{
		{
			name: "Failure: volume id is empty",
			nodeUnpublishVolReq: csi.NodeUnpublishVolumeRequest{
				VolumeId: "testvolid1",
			},
			wantsErr:           true,
			wantsRPCCode:       true,
			RPCCode:            codes.InvalidArgument,
			shouldRetryUnmount: true,
		},
		{
			name: "Failure: target path is empty",
			nodeUnpublishVolReq: csi.NodeUnpublishVolumeRequest{
				VolumeId: "testvolid1",
			},
			wantsErr:           true,
			wantsRPCCode:       true,
			RPCCode:            codes.InvalidArgument,
			shouldRetryUnmount: true,
		},
		{
			name: "Success for a mounted volume with a retry",
			nodeUnpublishVolReq: csi.NodeUnpublishVolumeRequest{
				VolumeId:   "testvolid1",
				TargetPath: tmpdir.New(t, "", `*mount`),
			},
			mountPoints:        []mount.MountPoint{},
			shouldRetryUnmount: true,
		},
	}
	s := scheme.Scheme
	s.AddKnownTypes(schema.GroupVersion{Group: v1alpha1.GroupVersion.Group, Version: v1alpha1.GroupVersion.Version},
		&v1alpha1.SecretProviderClass{},
		&v1alpha1.SecretProviderClassList{},
	)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.nodeUnpublishVolReq.TargetPath != "" {
				defer os.RemoveAll(test.nodeUnpublishVolReq.TargetPath)
			}
			if test.mountPoints != nil {
				// If file is a symlink, get its absolute path
				absFile, err := filepath.EvalSymlinks(test.nodeUnpublishVolReq.TargetPath)
				if err != nil {
					absFile = test.nodeUnpublishVolReq.TargetPath
				}
				test.mountPoints = append(test.mountPoints, mount.MountPoint{Path: absFile})
			}

			r := mocks.NewFakeReporter()
			ns, err := testNodeServer(t, test.mountPoints, fake.NewFakeClientWithScheme(s), r)
			if err != nil {
				t.Fatalf("expected error to be nil, got: %+v", err)
			}

			numberOfAttempts := 1
			// to ensure the remount is tried after previous failure and still fails
			if test.shouldRetryUnmount {
				numberOfAttempts = 2
			}

			for numberOfAttempts > 0 {
				// How many times 'total_node_unpublish' and 'total_node_unpublish_error' counters have been incremented so far
				c, cErr := r.ReportNodeUnPublishCtMetricInvoked(), r.ReportNodeUnPublishErrorCtMetricInvoked()

				_, err := ns.NodeUnpublishVolume(context.TODO(), &test.nodeUnpublishVolReq)
				if test.wantsErr && err == nil || !test.wantsErr && err != nil {
					t.Fatalf("expected err: %v, got: %+v", test.wantsErr, err)
				}

				// For the error cases where it is expected that the error will contain an RPC code
				gRPCStatus, ok := status.FromError(err)
				if test.wantsRPCCode && (!ok || test.RPCCode != gRPCStatus.Code()) {
					t.Fatalf("expected RPC status code: %v, got: %v\n", test.RPCCode, gRPCStatus.Code())
				}

				// Check that the correct counter has been incremented
				if err != nil && r.ReportNodeUnPublishErrorCtMetricInvoked() <= cErr {
					t.Fatalf("expected 'total_node_unpublish_error' counter to be incremented, but it was not")
				}
				if err == nil && r.ReportNodeUnPublishCtMetricInvoked() <= c {
					t.Fatalf("expected 'total_node_unpublish' counter to be incremented, but it was not")
				}

				numberOfAttempts--
			}
		})
	}
}
