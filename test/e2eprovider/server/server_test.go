// +build e2e

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

var (
	testMockServer *Server
	tempDir        = os.TempDir()
)

func TestMain(m *testing.M) {
	setup()
	exitCode := m.Run()
	teardown()
	os.Exit(exitCode)
}

func setup() {
	var err error

	testMockServer, err = NewE2EProviderServer(fmt.Sprintf("unix://%s/%s", tempDir, "e2e-provider.sock"))
	if err != nil {
		panic(err)
	}
}

func teardown() {
	testMockServer.Stop()
	os.Remove(fmt.Sprintf("%s/%s", tempDir, "e2e-provider.sock"))
}

func TestMockServer(t *testing.T) {
	cases := []struct {
		name string
	}{
		{
			name: "start",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := testMockServer.Start()
			if err != nil {
				t.Errorf("Did not expect error on server start: %v", err)
			}
		})
	}
}

func TestMount(t *testing.T) {
	cases := []struct {
		name    string
		input   *v1alpha1.MountRequest
		want    *v1alpha1.MountResponse
		wantErr error
	}{
		{
			"Parse static secrets",
			&v1alpha1.MountRequest{
				Attributes: func() string {
					attributes := map[string]string{
						"objects": `array:
  - |
    objectName: foo
    objectType: secret
  - |
    objectName: fookey
    objectType: key`,
					}
					data, _ := json.Marshal(attributes)
					return string(data)
				}(),
				Secrets:    "{}",
				Permission: "640",
				TargetPath: "/",
			},
			&v1alpha1.MountResponse{
				ObjectVersion: []*v1alpha1.ObjectVersion{
					{
						Id:      "secret/foo",
						Version: "v1",
					},
					{
						Id:      "secret/fookey",
						Version: "v1",
					},
				},
				Files: []*v1alpha1.File{
					{
						Path:     "foo",
						Contents: []byte("secret"),
					},
					{
						Path: "fookey",
						Contents: []byte(`-----BEGIN PUBLIC KEY-----
This is mock key
-----END PUBLIC KEY-----`),
					},
				},
			},
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := testMockServer.Mount(context.Background(), tc.input)
			if tc.wantErr != nil {
				if err == nil {
					t.Errorf("Did not receive expected error: %v", tc.wantErr)
					return
				}
				if tc.wantErr.Error() != err.Error() {
					t.Errorf("Received unexpected error: wanted %v, got %v", tc.wantErr, err)
					return
				}
			} else {
				if err != nil {
					t.Errorf("Received unexpected error: got %v", err)
					return
				}
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tc.want)
			if !reflect.DeepEqual(gotJSON, wantJSON) {
				t.Errorf("Didn't get expected results: wanted \n%s\n    got \n%s", string(wantJSON), string(gotJSON))
			}
		})
	}
}

func TestRotation(t *testing.T) {
	cases := []struct {
		name    string
		input   *v1alpha1.MountRequest
		want    *v1alpha1.MountResponse
		wantErr error
	}{
		{
			"Parse rotated secrets",
			&v1alpha1.MountRequest{
				Attributes: func() string {
					attributes := map[string]string{
						"objects": `array:
  - |
    objectName: foo
    objectType: secret
  - |
    objectName: fookey
    objectType: key`,
					}
					data, _ := json.Marshal(attributes)
					return string(data)
				}(),
				Secrets:    "{}",
				Permission: "640",
				TargetPath: "/",
			},
			&v1alpha1.MountResponse{
				ObjectVersion: []*v1alpha1.ObjectVersion{
					{
						Id:      "secret/foo",
						Version: "v2",
					},
					{
						Id:      "secret/fookey",
						Version: "v2",
					},
				},
				Files: []*v1alpha1.File{
					{
						Path:     "foo",
						Contents: []byte("rotated"),
					},
					{
						Path:     "fookey",
						Contents: []byte("rotated"),
					},
				},
			},
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := testMockServer.Mount(context.Background(), tc.input)
			if tc.wantErr != nil {
				if err == nil {
					t.Errorf("Did not receive expected error: %v", tc.wantErr)
					return
				}
				if tc.wantErr.Error() != err.Error() {
					t.Errorf("Received unexpected error: wanted %v, got %v", tc.wantErr, err)
					return
				}
			} else {
				if err != nil {
					t.Errorf("Received unexpected error: got %v", err)
					return
				}
			}

			// Rotate the secret
			got, err := testMockServer.Mount(context.Background(), tc.input)
			if tc.wantErr != nil {
				if err == nil {
					t.Errorf("Did not receive expected error: %v", tc.wantErr)
					return
				}
				if tc.wantErr.Error() != err.Error() {
					t.Errorf("Received unexpected error: wanted %v, got %v", tc.wantErr, err)
					return
				}
			} else {
				if err != nil {
					t.Errorf("Received unexpected error: got %v", err)
					return
				}
			}

			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tc.want)
			if !reflect.DeepEqual(gotJSON, wantJSON) {
				t.Errorf("Didn't get expected results: wanted \n%s\n    got \n%s", string(wantJSON), string(gotJSON))
			}
		})
	}
}
