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
	"os"
	"reflect"
	"sync"
	"testing"

	"sigs.k8s.io/secrets-store-csi-driver/provider/fake"
)

func getTempTestDir(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "ut")
	if err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}
	cleanup := func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fatalf("error cleaning up tmp dir: %s", err)
		}
	}
	return tmpDir, cleanup
}

func fakeServer(t *testing.T, path, provider string) (*fake.MockCSIProviderServer, func()) {
	t.Helper()
	serverEndpoint := fmt.Sprintf("%s/%s.sock", path, provider)
	server, err := fake.NewMocKCSIProviderServer(serverEndpoint)
	if err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}

	cleanup := func() {
		server.Stop()
		os.Remove(serverEndpoint)
	}

	return server, cleanup
}

func TestMountContent(t *testing.T) {
	cases := []struct {
		name                  string
		providerName          string
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
			attributes:            "{}",
			targetPath:            "/var/lib/kubelet/pods/d448c6a2-cda8-42e3-84fb-3cf75faa8399/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			permission:            "420",
			secrets:               "{}",
			expectedObjectVersion: map[string]string{"secret/secret1": "v1", "secret/secret2": "v2"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			socketPath, tclean := getTempTestDir(t)
			defer tclean()

			pool := NewPluginClientBuilder(socketPath)
			defer pool.Cleanup()

			server, cleanup := fakeServer(t, socketPath, test.providerName)
			defer cleanup()

			server.SetReturnError(test.providerError)
			server.SetObjects(test.expectedObjectVersion)
			server.SetProviderErrorCode(test.expectedErrorCode)
			server.Start()

			client, err := pool.Get(context.Background(), test.providerName)
			if err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}

			objectVersions, errorCode, err := MountContent(context.TODO(), client, test.attributes, test.secrets, test.targetPath, test.permission, nil)
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
			expectedErrorCode: "GRPCProviderError",
		},
		{
			name:              "missing target path in mount request",
			attributes:        "{}",
			expectedErrorCode: "GRPCProviderError",
		},
		{
			name:              "missing permission in mount request",
			attributes:        "{}",
			targetPath:        "/var/lib/kubelet/pods/d448c6a2-cda8-42e3-84fb-3cf75faa8399/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			expectedErrorCode: "GRPCProviderError",
		},
		{
			name:              "provider error for mount request",
			attributes:        "{}",
			targetPath:        "/var/lib/kubelet/pods/d448c6a2-cda8-42e3-84fb-3cf75faa8399/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			permission:        "0644",
			expectedErrorCode: "GRPCProviderError",
		},
		{
			name:              "missing object versions in mount response",
			attributes:        "{}",
			targetPath:        "/var/lib/kubelet/pods/d448c6a2-cda8-42e3-84fb-3cf75faa8399/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			permission:        "0644",
			expectedErrorCode: "GRPCProviderError",
		},
		{
			name:              "provider error",
			attributes:        "{}",
			targetPath:        "/var/lib/kubelet/pods/d448c6a2-cda8-42e3-84fb-3cf75faa8399/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			permission:        "0644",
			providerError:     errors.New("failed in provider"),
			expectedErrorCode: "GRPCProviderError",
		},
		{
			name:              "provider returns error code",
			attributes:        "{}",
			targetPath:        "/var/lib/kubelet/pods/d448c6a2-cda8-42e3-84fb-3cf75faa8399/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			permission:        "420",
			secrets:           "{}",
			expectedErrorCode: "AuthenticationFailed",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			socketPath, tclean := getTempTestDir(t)
			defer tclean()

			pool := NewPluginClientBuilder(socketPath)
			defer pool.Cleanup()

			providerName := "provider1"

			server, cleanup := fakeServer(t, socketPath, providerName)
			defer cleanup()

			server.SetReturnError(test.providerError)
			server.SetObjects(test.expectedObjectVersion)
			server.SetProviderErrorCode(test.expectedErrorCode)
			server.Start()

			client, err := pool.Get(context.Background(), providerName)
			if err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}

			objectVersions, errorCode, err := MountContent(context.TODO(), client, test.attributes, test.secrets, test.targetPath, test.permission, nil)
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

func TestPluginClientBuilder(t *testing.T) {
	path, tclean := getTempTestDir(t)
	defer tclean()

	cb := NewPluginClientBuilder(path)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		server, cleanup := fakeServer(t, path, fmt.Sprintf("server-%d", i))
		defer cleanup()
		server.Start()
	}

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		provider := fmt.Sprintf("server-%d", i)
		go func() {
			defer wg.Done()
			if _, err := cb.Get(ctx, provider); err != nil {
				t.Errorf("Get(%q) = %v, want nil", provider, err)
			}
		}()
	}

	wg.Wait()
}

func TestPluginClientBuilder_ConcurrentGet(t *testing.T) {
	path, tclean := getTempTestDir(t)
	defer tclean()

	cb := NewPluginClientBuilder(path)
	ctx := context.Background()

	provider := "server"
	server, cleanup := fakeServer(t, path, provider)
	defer cleanup()
	server.Start()

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cb.Get(ctx, provider); err != nil {
				t.Errorf("Get(%q) = %v, want nil", provider, err)
			}
		}()
	}

	wg.Wait()
}

func TestPluginClientBuilderErrorNotFound(t *testing.T) {
	path, tclean := getTempTestDir(t)
	defer tclean()

	cb := NewPluginClientBuilder(path)
	ctx := context.Background()

	if _, err := cb.Get(ctx, "notfoundprovider"); errors.Unwrap(err) != ErrProviderNotFound {
		t.Errorf("Get(%s) = %v, want %v", "notfoundprovider", err, ErrProviderNotFound)
	}

	// check that the provider is found once server is started
	server, cleanup := fakeServer(t, path, "notfoundprovider")
	defer cleanup()
	server.Start()

	if _, err := cb.Get(ctx, "notfoundprovider"); err != nil {
		t.Errorf("Get(%s) = %v, want nil", "notfoundprovider", err)
	}
}

func TestPluginClientBuilderErrorInvalid(t *testing.T) {
	path, tclean := getTempTestDir(t)
	defer tclean()

	cb := NewPluginClientBuilder(path)
	ctx := context.Background()

	if _, err := cb.Get(ctx, "bad/provider/name"); errors.Unwrap(err) != ErrInvalidProvider {
		t.Errorf("Get(%s) = %v, want %v", "bad/provider/name", err, ErrInvalidProvider)
	}
}
