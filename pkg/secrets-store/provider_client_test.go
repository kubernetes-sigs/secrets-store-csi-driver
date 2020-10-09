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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"sigs.k8s.io/secrets-store-csi-driver/provider/fake"
)

func getTempTestDir(t *testing.T) string {
	tmpDir, err := ioutil.TempDir("", "ut")
	if err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}
	return tmpDir
}

func TestMountContent(t *testing.T) {
	cases := []struct {
		name                  string
		providerName          CSIProviderName
		socketPath            string
		attributes            string
		secrets               string
		targetPath            string
		permission            string
		expectedObjectVersion map[string]string
		providerError         error
		expectedErrorCode     string
	}{
		{
			name:                  "provider successful response",
			providerName:          "provider1",
			socketPath:            getTempTestDir(t),
			attributes:            "{}",
			targetPath:            "/var/lib/kubelet/pods/d448c6a2-cda8-42e3-84fb-3cf75faa8399/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			permission:            "420",
			secrets:               "{}",
			expectedObjectVersion: map[string]string{"secret/secret1": "v1", "secret/secret2": "v2"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewProviderClient(test.providerName, test.socketPath)
			if err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}
			serverEndpoint := fmt.Sprintf("%s/%s.sock", test.socketPath, test.providerName)
			defer os.Remove(serverEndpoint)

			server, err := fake.NewMocKCSIProviderServer(serverEndpoint)
			if err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}
			server.SetReturnError(test.providerError)
			server.SetObjects(test.expectedObjectVersion)
			server.SetProviderErrorCode(test.expectedErrorCode)
			server.Start()

			objectVersions, errorCode, err := client.MountContent(context.TODO(), test.attributes, test.secrets, test.targetPath, test.permission, nil)
			if err != nil {
				t.Errorf("expected err to be nil, got: %+v", err)
			}
			if errorCode != test.expectedErrorCode {
				t.Errorf("expected error code: %v, got: %+v", test.expectedErrorCode, errorCode)
			}
			if test.expectedObjectVersion != nil && !reflect.DeepEqual(test.expectedObjectVersion, objectVersions) {
				t.Errorf("expected object versions: %v, got: %+v", test.expectedObjectVersion, objectVersions)
			}
		})
	}
}

func TestMountContentError(t *testing.T) {
	cases := []struct {
		name                  string
		providerName          CSIProviderName
		socketPath            string
		attributes            string
		secrets               string
		targetPath            string
		permission            string
		expectedObjectVersion map[string]string
		providerError         error
		expectedErrorCode     string
	}{
		{
			name:              "missing attributes in mount request",
			providerName:      "provider1",
			socketPath:        getTempTestDir(t),
			expectedErrorCode: "GRPCProviderError",
		},
		{
			name:              "missing target path in mount request",
			providerName:      "provider1",
			socketPath:        getTempTestDir(t),
			attributes:        "{}",
			expectedErrorCode: "GRPCProviderError",
		},
		{
			name:              "missing permission in mount request",
			providerName:      "provider1",
			socketPath:        getTempTestDir(t),
			attributes:        "{}",
			targetPath:        "/var/lib/kubelet/pods/d448c6a2-cda8-42e3-84fb-3cf75faa8399/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			expectedErrorCode: "GRPCProviderError",
		},
		{
			name:              "provider error for mount request",
			providerName:      "provider1",
			socketPath:        getTempTestDir(t),
			attributes:        "{}",
			targetPath:        "/var/lib/kubelet/pods/d448c6a2-cda8-42e3-84fb-3cf75faa8399/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			permission:        "0644",
			expectedErrorCode: "GRPCProviderError",
		},
		{
			name:              "missing object versions in mount response",
			providerName:      "provider1",
			socketPath:        getTempTestDir(t),
			attributes:        "{}",
			targetPath:        "/var/lib/kubelet/pods/d448c6a2-cda8-42e3-84fb-3cf75faa8399/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			permission:        "0644",
			expectedErrorCode: "GRPCProviderError",
		},
		{
			name:              "provider error",
			providerName:      "provider1",
			socketPath:        getTempTestDir(t),
			attributes:        "{}",
			targetPath:        "/var/lib/kubelet/pods/d448c6a2-cda8-42e3-84fb-3cf75faa8399/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			permission:        "0644",
			providerError:     errors.New("failed in provider"),
			expectedErrorCode: "GRPCProviderError",
		},
		{
			name:              "provider returns error code",
			providerName:      "provider1",
			socketPath:        getTempTestDir(t),
			attributes:        "{}",
			targetPath:        "/var/lib/kubelet/pods/d448c6a2-cda8-42e3-84fb-3cf75faa8399/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			permission:        "420",
			secrets:           "{}",
			expectedErrorCode: "AuthenticationFailed",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewProviderClient(test.providerName, test.socketPath)
			if err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}
			serverEndpoint := fmt.Sprintf("%s/%s.sock", test.socketPath, test.providerName)
			defer os.Remove(serverEndpoint)

			server, err := fake.NewMocKCSIProviderServer(serverEndpoint)
			if err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}
			server.SetReturnError(test.providerError)
			server.SetObjects(test.expectedObjectVersion)
			server.SetProviderErrorCode(test.expectedErrorCode)
			server.Start()

			objectVersions, errorCode, err := client.MountContent(context.TODO(), test.attributes, test.secrets, test.targetPath, test.permission, nil)
			if err == nil {
				t.Errorf("expected err to be not nil")
			}
			if errorCode != test.expectedErrorCode {
				t.Errorf("expected error code: %v, got: %+v", test.expectedErrorCode, errorCode)
			}
			if test.expectedObjectVersion != nil && !reflect.DeepEqual(test.expectedObjectVersion, objectVersions) {
				t.Errorf("expected object versions: %v, got: %+v", test.expectedObjectVersion, objectVersions)
			}
		})
	}
}
