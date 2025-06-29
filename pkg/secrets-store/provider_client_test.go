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
	"runtime"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/constants"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/fileutil"
	"sigs.k8s.io/secrets-store-csi-driver/provider/fake"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		skipon        string
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
					Mode:     0666,
					Contents: []byte("foo"),
				},
			},
			objectVersions: map[string]string{"foo": "v1"},
			expectedFiles: map[string]os.FileMode{
				"foo": 0666,
			},
		},
		{
			name:       "provider response with multiple files",
			permission: "777",
			files: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0666,
					Contents: []byte("foo"),
				},
				{
					Path:     "bar",
					Mode:     0444,
					Contents: []byte("bar"),
				},
			},
			objectVersions: map[string]string{"foo": "v1"},
			expectedFiles: map[string]os.FileMode{
				"foo": 0666,
				"bar": 0444,
			},
		},
		{
			name:       "provider response with nested files (linux)",
			permission: "777",
			files: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0644,
					Contents: []byte("foo"),
				},
				{
					Path:     "baz/bar",
					Mode:     0777,
					Contents: []byte("bar"),
				},
				{
					Path:     "baz/qux",
					Mode:     0777,
					Contents: []byte("qux"),
				},
			},
			objectVersions: map[string]string{"foo": "v1"},
			expectedFiles: map[string]os.FileMode{
				"foo":     0644,
				"baz/bar": 0777,
				"baz/qux": 0777,
			},
			skipon: "windows",
		},
		{
			// note: this is a bit weird because the path `baz\bar` on windows
			// should be a file `bar` nested in a folder `baz`. it _actually_
			// works on linux though because the `\` character is just treated
			// as part of the filename.
			name:       "provider response with nested files (windows)",
			permission: "777",
			files: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0444,
					Contents: []byte("foo"),
				},
				{
					Path:     "baz\\bar",
					Mode:     0444,
					Contents: []byte("bar"),
				},
				{
					Path:     "baz\\qux",
					Mode:     0666,
					Contents: []byte("qux"),
				},
			},
			objectVersions: map[string]string{"foo": "v1"},
			expectedFiles: map[string]os.FileMode{
				"foo":      0444,
				"baz\\bar": 0444,
				"baz\\qux": 0666,
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if test.skipon == runtime.GOOS {
				t.SkipNow()
			}
			socketPath := t.TempDir()
			targetPath := t.TempDir()

			pool := NewPluginClientBuilder([]string{socketPath})
			defer pool.Cleanup()

			server, cleanup := fakeServer(t, socketPath, "provider1")
			defer cleanup()

			server.SetReturnError(test.providerError)
			server.SetObjects(test.objectVersions)
			server.SetFiles(test.files)
			if err := server.Start(); err != nil {
				t.Fatalf("unable to start server :%s", err)
			}

			client, err := pool.Get(context.Background(), "provider1")
			if err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}

			objectVersions, _, err := MountContent(context.TODO(), client, "{}", "{}", targetPath, test.permission, nil, constants.NoGID)
			if err != nil {
				t.Errorf("expected err to be nil, got: %+v", err)
			}
			if test.objectVersions != nil && !reflect.DeepEqual(test.objectVersions, objectVersions) {
				t.Errorf("expected object versions: %v, got: %+v", test.objectVersions, objectVersions)
			}

			// check that file was written
			gotFiles := make(map[string]os.FileMode)
			paths, err := fileutil.GetMountedFiles(targetPath)
			if err != nil {
				t.Fatalf("unable to read mounted files: %s", err)
			}
			for rel, abs := range paths {
				info, err := os.Lstat(abs)
				if err != nil {
					t.Fatalf("unable to read mounted files: %s", err)
				}
				gotFiles[rel] = info.Mode()
			}

			if diff := cmp.Diff(test.expectedFiles, gotFiles); diff != "" {
				t.Errorf("MountContent() file mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMountContent_TooLarge(t *testing.T) {
	socketPath := t.TempDir()
	targetPath := t.TempDir()

	// set a very small max message size
	pool := NewPluginClientBuilder([]string{socketPath}, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(5)))
	defer pool.Cleanup()

	server, cleanup := fakeServer(t, socketPath, "provider1")
	defer cleanup()

	server.SetObjects(map[string]string{"foo": "v1"})
	server.SetFiles([]*v1alpha1.File{
		{
			Path:     "foo",
			Mode:     0644,
			Contents: []byte("foo"),
		},
	})
	if err := server.Start(); err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}

	client, err := pool.Get(context.Background(), "provider1")
	if err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}

	// rpc error: code = ResourceExhausted desc = grpc: received message larger than max (28 vs. 5)
	_, errorCode, err := MountContent(context.TODO(), client, "{}", "{}", targetPath, "777", nil, constants.NoGID)
	if err == nil {
		t.Errorf("expected err to be not nil")
	}
	if want := codes.ResourceExhausted; status.Code(err) != want {
		t.Errorf("expected error code: %v, got: %+v", want, status.Code(err))
	}
	if want := "GRPCProviderError"; errorCode != want {
		t.Errorf("expected error code: %v, got: %+v", want, errorCode)
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
			socketPath := t.TempDir()

			pool := NewPluginClientBuilder([]string{socketPath})
			defer pool.Cleanup()

			providerName := "provider1"

			server, cleanup := fakeServer(t, socketPath, providerName)
			defer cleanup()

			server.SetReturnError(test.providerError)
			server.SetObjects(test.expectedObjectVersion)
			server.SetProviderErrorCode(test.expectedErrorCode)
			if err := server.Start(); err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}

			client, err := pool.Get(context.Background(), providerName)
			if err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}

			objectVersions, errorCode, err := MountContent(context.TODO(), client, test.attributes, test.secrets, test.targetPath, test.permission, nil, constants.NoGID)
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
	path := t.TempDir()

	cb := NewPluginClientBuilder([]string{path})
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		server, cleanup := fakeServer(t, path, fmt.Sprintf("server-%d", i))
		defer cleanup()
		if err := server.Start(); err != nil {
			t.Fatalf("expected err to be nil, got: %+v", err)
		}
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

func TestPluginClientBuilderMultiPath(t *testing.T) {
	emptyPath := t.TempDir()
	path := t.TempDir()

	// Ensure that if the path containing the listening socket is not the first
	// path checked that the operation still succeeds.
	cb := NewPluginClientBuilder([]string{emptyPath, path})
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		server, cleanup := fakeServer(t, path, fmt.Sprintf("server-%d", i))
		defer cleanup()
		if err := server.Start(); err != nil {
			t.Fatalf("expected err to be nil, got: %+v", err)
		}
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
	path := t.TempDir()

	cb := NewPluginClientBuilder([]string{path})
	ctx := context.Background()

	provider := "server"
	server, cleanup := fakeServer(t, path, provider)
	defer cleanup()
	if err := server.Start(); err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}

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
	path := t.TempDir()

	cb := NewPluginClientBuilder([]string{path})
	ctx := context.Background()

	if _, err := cb.Get(ctx, "notfoundprovider"); !errors.Is(err, errProviderNotFound) {
		t.Errorf("Get(%s) = %v, want %v", "notfoundprovider", err, errProviderNotFound)
	}

	// check that the provider is found once server is started
	server, cleanup := fakeServer(t, path, "notfoundprovider")
	defer cleanup()
	if err := server.Start(); err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}

	if _, err := cb.Get(ctx, "notfoundprovider"); err != nil {
		t.Errorf("Get(%s) = %v, want nil", "notfoundprovider", err)
	}
}

func TestPluginClientBuilderErrorInvalid(t *testing.T) {
	path := t.TempDir()

	cb := NewPluginClientBuilder([]string{path})
	ctx := context.Background()

	if _, err := cb.Get(ctx, "bad/provider/name"); !errors.Is(err, errInvalidProvider) {
		t.Errorf("Get(%s) = %v, want %v", "bad/provider/name", err, errInvalidProvider)
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
			socketPath := t.TempDir()

			pool := NewPluginClientBuilder([]string{socketPath})
			defer pool.Cleanup()

			server, cleanup := fakeServer(t, socketPath, "provider1")
			defer cleanup()

			if err := server.Start(); err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}

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
	path := t.TempDir()

	cb := NewPluginClientBuilder([]string{path})
	ctx := context.Background()
	healthCheckInterval := 1 * time.Millisecond

	provider := "server"
	server, cleanup := fakeServer(t, path, provider)
	defer cleanup()
	if err := server.Start(); err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}

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

func TestIsMaxRecvMsgSizeError(t *testing.T) {
	cases := []struct {
		name string
		// inputs
		err error
		// expectations
		want bool
	}{
		{
			name: "not resource exhausted error",
			err:  errors.New("failed to mount"),
			want: false,
		},
		{
			name: "generic resource exhausted error",
			err:  status.Errorf(codes.ResourceExhausted, "user quota exceeded"),
			want: false,
		},
		{
			name: "resource exhausted error because of quota propagation",
			err:  status.Errorf(codes.ResourceExhausted, "grpc: received message larger than max length allowed on current machine"),
			want: false,
		},
		{
			name: "resource exhausted error because message larger than max length",
			err:  status.Errorf(codes.ResourceExhausted, "grpc: received message larger than max"),
			want: true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := isMaxRecvMsgSizeError(test.err)
			if got != test.want {
				t.Errorf("isMaxRecvMsgSizeError() = %v, want %v", got, test.want)
			}
		})
	}
}
