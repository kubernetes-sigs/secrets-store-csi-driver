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
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/k8s"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store/mocks"
	providerfake "sigs.k8s.io/secrets-store-csi-driver/provider/fake"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	mount "k8s.io/mount-utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testNodeServer(t *testing.T, client client.Client, reporter StatsReporter) (*nodeServer, error) {
	t.Helper()

	// Create a mock provider named "provider1".
	socketPath := t.TempDir()
	server, err := providerfake.NewMocKCSIProviderServer(filepath.Join(socketPath, "provider1.sock"))
	if err != nil {
		t.Fatalf("unexpected mock provider failure: %v", err)
	}
	server.SetObjects(map[string]string{"secret/object1": "v2"})
	if err := server.Start(); err != nil {
		t.Fatalf("unexpected mock provider start failure: %v", err)
	}
	t.Cleanup(server.Stop)

	providerClients := NewPluginClientBuilder([]string{socketPath})
	return newNodeServer("testnode", mount.NewFakeMounter([]mount.MountPoint{}), providerClients, client, client, reporter, k8s.NewTokenClient(fakeclient.NewSimpleClientset(), "test-driver", 1*time.Second))
}

func getInitObjects(customize func(*secretsstorev1.SecretProviderClass)) []client.Object {
	var spc = &secretsstorev1.SecretProviderClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "provider1",
			Namespace: "default",
		},
		Spec: secretsstorev1.SecretProviderClassSpec{
			Provider:   "provider1",
			Parameters: map[string]string{"parameter1": "value1"},
		},
	}
	customize(spc)
	var initObjects = []client.Object{
		spc,
	}
	return initObjects
}

func getRequest(t *testing.T, customize func(*csi.NodePublishVolumeRequest)) *csi.NodePublishVolumeRequest {
	var request = &csi.NodePublishVolumeRequest{
		VolumeCapability: &csi.VolumeCapability{},
		VolumeId:         "testvolid1",
		VolumeContext: map[string]string{
			"secretProviderClass": "provider1",
			CSIPodName:            "pod1",
			CSIPodNamespace:       "default",
			CSIPodUID:             "poduid1",
		},
		TargetPath: targetPath(t),
		Readonly:   true,
	}
	customize(request)
	return request
}
func TestNodePublishVolume_Errors(t *testing.T) {
	tests := []struct {
		name              string
		nodePublishVolReq *csi.NodePublishVolumeRequest
		initObjects       []client.Object
		want              codes.Code
	}{
		{
			name:              "volume capabilities nil",
			nodePublishVolReq: &csi.NodePublishVolumeRequest{},
			want:              codes.InvalidArgument,
		},
		{
			name: "volume id is empty",
			nodePublishVolReq: &csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "target path is empty",
			nodePublishVolReq: &csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "volume context is not set",
			nodePublishVolReq: &csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       targetPath(t),
			},
			want: codes.InvalidArgument,
		},
		{
			name: "secret provider class not found",
			nodePublishVolReq: &csi.NodePublishVolumeRequest{
				VolumeCapability: &csi.VolumeCapability{},
				VolumeId:         "testvolid1",
				TargetPath:       targetPath(t),
				VolumeContext:    map[string]string{"secretProviderClass": "provider1"},
			},
			want: codes.Unknown,
		},
		{
			name:              "spc missing",
			nodePublishVolReq: getRequest(t, func(*csi.NodePublishVolumeRequest) {}),
			initObjects: getInitObjects(func(s *secretsstorev1.SecretProviderClass) {
				s.ObjectMeta.Namespace = "incorrect_namespace"
			}),
			want: codes.Unknown,
		},
		{
			name:              "provider not set in secret provider class",
			nodePublishVolReq: getRequest(t, func(*csi.NodePublishVolumeRequest) {}),
			initObjects: getInitObjects(func(s *secretsstorev1.SecretProviderClass) {
				s.Spec = secretsstorev1.SecretProviderClassSpec{}
			}),
			want: codes.Unknown,
		},
		{
			name:              "parameters not set in secret provider class",
			nodePublishVolReq: getRequest(t, func(*csi.NodePublishVolumeRequest) {}),
			initObjects: getInitObjects(func(s *secretsstorev1.SecretProviderClass) {
				s.Spec.Parameters = map[string]string{}
			}),
			want: codes.Unknown,
		},
		{
			name: "read only is not set to true",
			nodePublishVolReq: getRequest(t, func(r *csi.NodePublishVolumeRequest) {
				r.Readonly = false
			}),
			initObjects: getInitObjects(func(*secretsstorev1.SecretProviderClass) {}),
			want:        codes.InvalidArgument,
		},
		{
			name:              "provider not installed",
			nodePublishVolReq: getRequest(t, func(*csi.NodePublishVolumeRequest) {}),
			initObjects: getInitObjects(func(s *secretsstorev1.SecretProviderClass) {
				s.Spec.Provider = "provider_not_installed"
			}),
			want: codes.Unknown,
		},
		{
			name: "Invalid FSGroup",
			nodePublishVolReq: getRequest(t, func(r *csi.NodePublishVolumeRequest) {
				r.VolumeCapability = &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							VolumeMountGroup: "INVALID",
						},
					},
				}
			}),
			initObjects: getInitObjects(func(*secretsstorev1.SecretProviderClass) {}),
			want:        codes.InvalidArgument,
		},
	}

	s := scheme.Scheme
	s.AddKnownTypes(schema.GroupVersion{Group: secretsstorev1.GroupVersion.Group, Version: secretsstorev1.GroupVersion.Version},
		&secretsstorev1.SecretProviderClass{},
		&secretsstorev1.SecretProviderClassList{},
		&secretsstorev1.SecretProviderClassPodStatus{},
	)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := mocks.NewFakeReporter()

			ns, err := testNodeServer(t, fake.NewClientBuilder().WithScheme(s).WithObjects(test.initObjects...).Build(), r)
			if err != nil {
				t.Fatalf("expected error to be nil, got: %+v", err)
			}

			for attempts := 2; attempts > 0; attempts-- {
				// How many times 'total_node_publish' and 'total_node_publish_error' counters have been incremented so far.
				c, cErr := r.ReportNodePublishCtMetricInvoked(), r.ReportNodePublishErrorCtMetricInvoked()

				_, err = ns.NodePublishVolume(context.TODO(), test.nodePublishVolReq)
				if code := status.Code(err); code != test.want {
					t.Errorf("expected RPC status code: %v, got: %v\n\n %v", test.want, code, err)
				}

				// Check that the correct counter has been incremented.
				if err != nil && r.ReportNodePublishErrorCtMetricInvoked() <= cErr {
					t.Fatalf("expected 'total_node_publish_error' counter to be incremented, but it was not")
				}
				if err == nil && r.ReportNodePublishCtMetricInvoked() <= c {
					t.Fatalf("expected 'total_node_publish' counter to be incremented, but it was not")
				}

				// if theres not an error we should check the mount w
				mnts, err := ns.mounter.List()
				if err != nil {
					t.Fatalf("expected err to be nil, got: %v", err)
				}
				if len(mnts) != 0 {
					t.Errorf("NodePublishVolume returned an error, expected mount points to be 0: %v", mnts)
				}
			}
		})
	}
}

func TestNodePublishVolume(t *testing.T) {
	tests := []struct {
		name              string
		nodePublishVolReq *csi.NodePublishVolumeRequest
		initObjects       []client.Object
	}{
		{
			name:              "volume mount",
			nodePublishVolReq: getRequest(t, func(*csi.NodePublishVolumeRequest) {}),
			initObjects:       getInitObjects(func(*secretsstorev1.SecretProviderClass) {}),
		},
		{
			name: "volume mount with refresh token",
			nodePublishVolReq: getRequest(t, func(r *csi.NodePublishVolumeRequest) {
				// not a real token, just for testing
				r.VolumeContext[CSIPodServiceAccountTokens] = `{"https://kubernetes.default.svc":{"token":"eyJhbGciOiJSUzI1NiIsImtpZCI6IjEyMyJ9.eyJhdWQiOlsiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjIl0sImV4cCI6MTYxMTk1OTM5NiwiaWF0IjoxNjExOTU4Nzk2LCJpc3MiOiJodHRwczovL2t1YmVybmV0ZXMuZGVmYXVsdC5zdmMiLCJrdWJlcm5ldGVzLmlvIjp7Im5hbWVzcGFjZSI6ImRlZmF1bHQiLCJzZXJ2aWNlYWNjb3VudCI6eyJuYW1lIjoiZGVmYXVsdCIsInVpZCI6IjA5MWUyNTU3LWJkODYtNDhhMC1iZmNmLWI1YTI4ZjRjODAyNCJ9fSwibmJmIjoxNjExOTU4Nzk2LCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6ZGVmYXVsdDpkZWZhdWx0In0.YNU2Z_gEE84DGCt8lh9GuE8gmoof-Pk_7emp3fsyj9pq16DRiDaLtOdprH-njpOYqvtT5Uf_QspFc_RwD_pdq9UJWCeLxFkRTsYR5WSjhMFcl767c4Cwp_oZPYhaHd1x7aU1emH-9oarrM__tr1hSmGoAc2I0gUSkAYFueaTUSy5e5d9QKDfjVljDRc7Yrp6qAAfd1OuDdk1XYIjrqTHk1T1oqGGlcd3lRM_dKSsW5I_YqgKMrjwNt8yOKcdKBrgQhgC42GZbFDRVJDJHs_Hq32xo-2s3PJ8UZ_alN4wv8EbuwB987_FHBTc_XAULHPvp0mCv2C5h0V2A7gzccv30A","expirationTimestamp":"2021-01-29T22:29:56Z"}}`
				r.VolumeContext["providerName"] = "provider1"
			}),
			initObjects: getInitObjects(func(*secretsstorev1.SecretProviderClass) {}),
		},
		{
			name: "volume mount with valid FSGroup",
			nodePublishVolReq: getRequest(t, func(r *csi.NodePublishVolumeRequest) {
				r.VolumeCapability = &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{
							VolumeMountGroup: "1004",
						},
					},
				}
			}),
			initObjects: getInitObjects(func(*secretsstorev1.SecretProviderClass) {}),
		},
	}

	s := scheme.Scheme
	s.AddKnownTypes(schema.GroupVersion{Group: secretsstorev1.GroupVersion.Group, Version: secretsstorev1.GroupVersion.Version},
		&secretsstorev1.SecretProviderClass{},
		&secretsstorev1.SecretProviderClassList{},
		&secretsstorev1.SecretProviderClassPodStatus{},
	)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := mocks.NewFakeReporter()

			ns, err := testNodeServer(t, fake.NewClientBuilder().WithScheme(s).WithObjects(test.initObjects...).Build(), r)
			if err != nil {
				t.Fatalf("expected error to be nil, got: %+v", err)
			}

			for attempts := 2; attempts > 0; attempts-- {
				// How many times 'total_node_publish' and 'total_node_publish_error' counters have been incremented so far.
				c, cErr := r.ReportNodePublishCtMetricInvoked(), r.ReportNodePublishErrorCtMetricInvoked()

				_, err = ns.NodePublishVolume(context.TODO(), test.nodePublishVolReq)
				if code := status.Code(err); code != codes.OK {
					t.Errorf("expected RPC status code: %v, got: %v\n", codes.OK, err)
				}

				// Check that the correct counter has been incremented.
				if err != nil && r.ReportNodePublishErrorCtMetricInvoked() <= cErr {
					t.Fatalf("expected 'total_node_publish_error' counter to be incremented, but it was not")
				}
				if err == nil && r.ReportNodePublishCtMetricInvoked() <= c {
					t.Fatalf("expected 'total_node_publish' counter to be incremented, but it was not")
				}

				// if theres not an error we should check the mount w
				mnts, err := ns.mounter.List()
				if err != nil {
					t.Fatalf("expected err to be nil, got: %v", err)
				}
				if len(mnts) == 0 {
					t.Errorf("expected mounts...: %v", mnts)
				}
			}
		})
	}
}

func TestNodeUnpublishVolume(t *testing.T) {
	s := scheme.Scheme
	s.AddKnownTypes(schema.GroupVersion{Group: secretsstorev1.GroupVersion.Group, Version: secretsstorev1.GroupVersion.Version},
		&secretsstorev1.SecretProviderClass{},
		&secretsstorev1.SecretProviderClassList{},
	)

	r := mocks.NewFakeReporter()
	ns, err := testNodeServer(t, fake.NewClientBuilder().WithScheme(s).Build(), r)
	if err != nil {
		t.Fatalf("expected error to be nil, got: %+v", err)
	}

	req := &csi.NodeUnpublishVolumeRequest{
		VolumeId:   "testvolid1",
		TargetPath: targetPath(t),
	}

	// Create the fake mount first so that it can be unmounted.
	if err := ns.mounter.Mount("tmpfs", req.TargetPath, "tmpfs", []string{}); err != nil {
		t.Fatalf("unable to mount: %v", err)
	}

	// Add a file to the mount.
	if err := os.WriteFile(filepath.Join(req.TargetPath, "testfile.txt"), []byte("test"), 0600); err != nil {
		t.Fatalf("unable to add file to targetpath: %v", err)
	}

	// Repeat the request multiple times to ensure it consistently returns OK,
	// even if it has already been unmounted.
	for attempts := 2; attempts > 0; attempts-- {
		// How many times 'total_node_unpublish' and 'total_node_unpublish_error' counters have been incremented so far
		c, cErr := r.ReportNodeUnPublishCtMetricInvoked(), r.ReportNodeUnPublishErrorCtMetricInvoked()

		_, err := ns.NodeUnpublishVolume(context.TODO(), req)
		if code := status.Code(err); code != codes.OK {
			t.Errorf("expected RPC status code: %v, got: %v\n", codes.OK, err)
		}

		// Check that the correct counter has been incremented
		if err != nil && r.ReportNodeUnPublishErrorCtMetricInvoked() <= cErr {
			t.Fatalf("expected 'total_node_unpublish_error' counter to be incremented, but it was not")
		}
		if err == nil && r.ReportNodeUnPublishCtMetricInvoked() <= c {
			t.Fatalf("expected 'total_node_unpublish' counter to be incremented, but it was not")
		}

		// Ensure that the mounts were unmounted.
		mnts, err := ns.mounter.List()
		if err != nil {
			t.Fatalf("expected err to be nil, got: %v", err)
		}
		if len(mnts) != 0 {
			t.Errorf("NodeUnpublishVolume returned an error, expected mount points to be 0: %v", mnts)
		}
	}
}

func TestNodeUnpublishVolume_Error(t *testing.T) {
	tests := []struct {
		name                string
		nodeUnpublishVolReq *csi.NodeUnpublishVolumeRequest
		want                codes.Code
	}{
		{
			name: "Failure: volume id is empty",
			nodeUnpublishVolReq: &csi.NodeUnpublishVolumeRequest{
				TargetPath: targetPath(t),
			},
			want: codes.InvalidArgument,
		},
		{
			name: "Failure: target path is empty",
			nodeUnpublishVolReq: &csi.NodeUnpublishVolumeRequest{
				VolumeId: "testvolid1",
			},
			want: codes.InvalidArgument,
		},
	}
	s := scheme.Scheme
	s.AddKnownTypes(schema.GroupVersion{Group: secretsstorev1.GroupVersion.Group, Version: secretsstorev1.GroupVersion.Version},
		&secretsstorev1.SecretProviderClass{},
		&secretsstorev1.SecretProviderClassList{},
	)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := mocks.NewFakeReporter()
			ns, err := testNodeServer(t, fake.NewClientBuilder().WithScheme(s).Build(), r)
			if err != nil {
				t.Fatalf("expected error to be nil, got: %+v", err)
			}

			for attempts := 2; attempts > 0; attempts-- {
				// How many times 'total_node_unpublish' and 'total_node_unpublish_error' counters have been incremented so far
				c, cErr := r.ReportNodeUnPublishCtMetricInvoked(), r.ReportNodeUnPublishErrorCtMetricInvoked()

				_, err := ns.NodeUnpublishVolume(context.TODO(), test.nodeUnpublishVolReq)
				if code := status.Code(err); code != test.want {
					t.Errorf("expected RPC status code: %v, got: %v\n", test.want, code)
				}

				// Check that the correct counter has been incremented
				if err != nil && r.ReportNodeUnPublishErrorCtMetricInvoked() <= cErr {
					t.Fatalf("expected 'total_node_unpublish_error' counter to be incremented, but it was not")
				}
				if err == nil && r.ReportNodeUnPublishCtMetricInvoked() <= c {
					t.Fatalf("expected 'total_node_unpublish' counter to be incremented, but it was not")
				}
			}
		})
	}
}

// targetPath returns a tmp file path that looks like a valid target path.
func targetPath(t *testing.T) string {
	uid := "fake-uid"
	vol := "spc-volume"
	path := filepath.Join(t.TempDir(), "pods", uid, "volumes", "kubernetes.io~csi", vol, "mount")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}
	return path
}
