//go:build e2e
// +build e2e

/*
Copyright 2021 The Kubernetes Authors.

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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"

	"github.com/google/go-cmp/cmp"
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

	_, _ = testMockServer.Mount(context.Background(), mountRequest)
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

func TestValidateTokens(t *testing.T) {
	tokens := `{"aud1":{"token":"eyJhbGciOiJSUzI1NiIsImtpZCI6InRhVDBxbzhQVEZ1ajB1S3BYUUxIclRsR01XakxjemJNOTlzWVMxSlNwbWcifQ.eyJhdWQiOlsiYXBpOi8vQXp1cmVBRGlUb2tlbkV4Y2hhbmdlIl0sImV4cCI6MTY0MzIzNDY0NywiaWF0IjoxNjQzMjMxMDQ3LCJpc3MiOiJodHRwczovL2t1YmVybmV0ZXMuZGVmYXVsdC5zdmMuY2x1c3Rlci5sb2NhbCIsImt1YmVybmV0ZXMuaW8iOnsibmFtZXNwYWNlIjoidGVzdC12MWFscGhhMSIsInBvZCI6eyJuYW1lIjoic2VjcmV0cy1zdG9yZS1pbmxpbmUtY3JkIiwidWlkIjoiYjBlYmZjMzUtZjEyNC00ZTEyLWI3N2UtYjM0MjM2N2IyMDNmIn0sInNlcnZpY2VhY2NvdW50Ijp7Im5hbWUiOiJkZWZhdWx0IiwidWlkIjoiMjViNGY1NzgtM2U4MC00NTczLWJlOGQtZTdmNDA5ZDI0MmI2In19LCJuYmYiOjE2NDMyMzEwNDcsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDp0ZXN0LXYxYWxwaGExOmRlZmF1bHQifQ.ALE46aKmtTV7dsuFOwDZqvEjdHFUTNP-JVjMxexTemmPA78fmPTUZF0P6zANumA03fjX3L-MZNR3PxmEZgKA9qEGIDsljLsUWsVBEquowuBh8yoBYkGkMJmRfmbfS3y7_4Q7AU3D9Drw4iAHcn1GwedjOQC0i589y3dkNNqf8saqHfXkbSSLtSE0f2uzI-PjuTKvR1kuojEVNKlEcA4wsKfoiRpkua17sHkHU0q9zxCMDCr_1f8xbigRnRx0wscU3vy-8KhF3zQtpcWkk3r4C5YSXut9F3xjz5J9DUQn2vNMfZg4tOdcR-9Xv9fbY5iujiSlS58GEktSEa3SE9wrCw\",\"expirationTimestamp\":\"2022-01-26T22:04:07Z\"},\"gcp\":{\"token\":\"eyJhbGciOiJSUzI1NiIsImtpZCI6InRhVDBxbzhQVEZ1ajB1S3BYUUxIclRsR01XakxjemJNOTlzWVMxSlNwbWcifQ.eyJhdWQiOlsiZ2NwIl0sImV4cCI6MTY0MzIzNDY0NywiaWF0IjoxNjQzMjMxMDQ3LCJpc3MiOiJodHRwczovL2t1YmVybmV0ZXMuZGVmYXVsdC5zdmMuY2x1c3Rlci5sb2NhbCIsImt1YmVybmV0ZXMuaW8iOnsibmFtZXNwYWNlIjoidGVzdC12MWFscGhhMSIsInBvZCI6eyJuYW1lIjoic2VjcmV0cy1zdG9yZS1pbmxpbmUtY3JkIiwidWlkIjoiYjBlYmZjMzUtZjEyNC00ZTEyLWI3N2UtYjM0MjM2N2IyMDNmIn0sInNlcnZpY2VhY2NvdW50Ijp7Im5hbWUiOiJkZWZhdWx0IiwidWlkIjoiMjViNGY1NzgtM2U4MC00NTczLWJlOGQtZTdmNDA5ZDI0MmI2In19LCJuYmYiOjE2NDMyMzEwNDcsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDp0ZXN0LXYxYWxwaGExOmRlZmF1bHQifQ.BT0YGI7bGdSNaIBqIEnVL0Ky5t-fynaemSGxjGdKOPl0E22UIVGDpAMUhaS19i20c-Dqs-Kn0N-R5QyDNpZg8vOL5KIFqu2kSYNbKxtQW7TPYIsV0d9wUZjLSr54DKrmyXNMGRoT2bwcF4yyfmO46eMmZSaXN8Y4lgapeabg6CBVVQYHD-GrgXf9jVLeJfCQkTuojK1iXOphyD6NqlGtVCaY1jWxbBMibN0q214vKvQboub8YMuvclGdzn_l_ZQSTjvhBj9I-W1t-JArVjqHoIb8_FlR9BSgzgL7V3Jki55vmiOdEYqMErJWrIZPP3s8qkU5hhO9rSVEd3LJHponvQ","expirationTimestamp":"2022-01-26T22:04:07Z"}}` //nolint
	audiences := "aud1"

	if err := validateTokens(audiences, tokens); err != nil {
		t.Errorf("validateTokens() error = %v, wantErr nil", err)
	}
}

func TestValidateTokensError(t *testing.T) {
	tokens := ""
	audiences := "aud1,aud2"

	if err := validateTokens(audiences, tokens); err == nil {
		t.Errorf("validateTokens() error is nil, want error")
	}
}
