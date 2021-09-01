//go:build e2e
// +build e2e

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
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
	err := testMockServer.Start()
	if err != nil {
		t.Errorf("Did not expect error on server start: %v", err)
	}
}

func TestMount(t *testing.T) {
	mountRequest := &v1alpha1.MountRequest{
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
	}

	expectedMountResponse := &v1alpha1.MountResponse{
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
	}

	mountResponse, _ := testMockServer.Mount(context.Background(), mountRequest)
	gotJSON, _ := json.Marshal(mountResponse)
	wantJSON, _ := json.Marshal(expectedMountResponse)

	if diff := cmp.Diff(gotJSON, wantJSON); diff != "" {
		t.Errorf("didn't get expected results: (-want +got):\n%s", diff)
	}
}

func TestMountError(t *testing.T) {
	var wantError error
	mountRequest := &v1alpha1.MountRequest{
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
	}

	_, err := testMockServer.Mount(context.Background(), mountRequest)
	if wantError != nil {
		if err == nil {
			t.Errorf("did not receive expected error: got - %v\nwanted - %v", err, wantError)
			return
		}
		if wantError.Error() != err.Error() {
			t.Errorf("received unexpected error: got - %v\nwanted - %v", err, wantError)
			return
		}
	} else {
		if err != nil {
			t.Errorf("received unexpected error: got %v", err)
			return
		}
	}
}

func TestRotation(t *testing.T) {
	mountRequest := &v1alpha1.MountRequest{
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
	}

	expectedMountResponse := &v1alpha1.MountResponse{
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
	}

	testMockServer.Mount(context.Background(), mountRequest)
	// enable rotation response
	os.Setenv("ROTATION_ENABLED", "true")
	// Rotate the secret
	mountResponse, _ := testMockServer.Mount(context.Background(), mountRequest)

	gotJSON, _ := json.Marshal(mountResponse)
	wantJSON, _ := json.Marshal(expectedMountResponse)

	if diff := cmp.Diff(gotJSON, wantJSON); diff != "" {
		t.Errorf("didn't get expected results: (-want +got):\n%s", diff)
	}
}
