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
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/test_utils/tmpdir"
	"sigs.k8s.io/secrets-store-csi-driver/provider/fake"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

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
		name string
		// inputs
		permission string
		// mock outputs
		objectVersions map[string]string
		providerError  error
		files          []*v1alpha1.File
		// expectations
		expectedFiles map[string]os.FileMode
	}{
		{
			name:           "provider successful response (no files)",
			permission:     "420",
			objectVersions: map[string]string{"secret/secret1": "v1", "secret/secret2": "v2"},
			expectedFiles:  map[string]os.FileMode{},
		},
		{
			name:       "provider response with file",
			permission: "777",
			files: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0644,
					Contents: []byte("foo"),
				},
			},
			objectVersions: map[string]string{"foo": "v1"},
			expectedFiles: map[string]os.FileMode{
				"foo": 0644,
			},
		},
		{
			name:       "provider response with multiple files",
			permission: "777",
			files: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0644,
					Contents: []byte("foo"),
				},
				{
					Path:     "bar",
					Mode:     0777,
					Contents: []byte("bar"),
				},
			},
			objectVersions: map[string]string{"foo": "v1"},
			expectedFiles: map[string]os.FileMode{
				"foo": 0644,
				"bar": 0777,
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			socketPath := tmpdir.New(t, "", "ut")
			targetPath := tmpdir.New(t, "", "ut")

			pool := NewPluginClientBuilder(socketPath)
			defer pool.Cleanup()

			server, cleanup := fakeServer(t, socketPath, "provider1")
			defer cleanup()

			server.SetReturnError(test.providerError)
			server.SetObjects(test.objectVersions)
			server.SetFiles(test.files)
			server.Start()

			client, err := pool.Get(context.Background(), "provider1")
			if err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}

			objectVersions, _, err := MountContent(context.TODO(), client, "{}", "{}", targetPath, test.permission, nil)
			if err != nil {
				t.Errorf("expected err to be nil, got: %+v", err)
			}
			if test.objectVersions != nil && !reflect.DeepEqual(test.objectVersions, objectVersions) {
				t.Errorf("expected object versions: %v, got: %+v", test.objectVersions, objectVersions)
			}

			// check that file was written
			gotFiles := make(map[string]os.FileMode)
			filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
				// skip mount folder
				if path == targetPath {
					return nil
				}
				rel, err := filepath.Rel(targetPath, path)
				if err != nil {
					return err
				}
				gotFiles[rel] = info.Mode()
				return nil
			})

			if diff := cmp.Diff(test.expectedFiles, gotFiles); diff != "" {
				t.Errorf("MountContent() file mismatch (-want +got):\n%s", diff)
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
			socketPath := tmpdir.New(t, "", "ut")

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
	path := tmpdir.New(t, "", "ut")

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
	path := tmpdir.New(t, "", "ut")

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
	path := tmpdir.New(t, "", "ut")

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
	path := tmpdir.New(t, "", "ut")

	cb := NewPluginClientBuilder(path)
	ctx := context.Background()

	if _, err := cb.Get(ctx, "bad/provider/name"); errors.Unwrap(err) != ErrInvalidProvider {
		t.Errorf("Get(%s) = %v, want %v", "bad/provider/name", err, ErrInvalidProvider)
	}
}

func TestVersion(t *testing.T) {
	cases := []struct {
		name                   string
		expectedRuntimeVersion string
	}{
		{
			name:                   "provider version successful response",
			expectedRuntimeVersion: "0.0.10",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			socketPath := tmpdir.New(t, "", "ut")

			pool := NewPluginClientBuilder(socketPath)
			defer pool.Cleanup()

			server, cleanup := fakeServer(t, socketPath, "provider1")
			defer cleanup()

			server.Start()

			client, err := pool.Get(context.Background(), "provider1")
			if err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}

			runtimeVersion, err := Version(context.TODO(), client)
			if err != nil {
				t.Errorf("expected err to be nil, got: %+v", err)
			}
			if test.expectedRuntimeVersion != runtimeVersion {
				t.Errorf("expected version: %s, got: %s", test.expectedRuntimeVersion, runtimeVersion)
			}
		})
	}
}

func TestPluginClientBuilder_HealthCheck(t *testing.T) {
	// this test asserts the read lock and unlock semantics in the
	// HealthCheck() method work as expected
	path := tmpdir.New(t, "", "ut")

	cb := NewPluginClientBuilder(path)
	ctx := context.Background()
	healthCheckInterval := 1 * time.Millisecond

	provider := "server"
	server, cleanup := fakeServer(t, path, provider)
	defer cleanup()
	server.Start()

	// run the provider healthcheck
	go cb.HealthCheck(ctx, healthCheckInterval)
	var wg sync.WaitGroup

	// try a concurrent get with the healthcheck running in the background
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
