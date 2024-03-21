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

package fakeprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"

	"google.golang.org/grpc"
)

type MockCSIDriverProviderClient struct {
	socketPath string
	returnErr  error
	errorCode  string
	objects    []*v1alpha1.ObjectVersion
	files      []*v1alpha1.File
}

// NewMocKCSIDriverProviderClient returns a mock csi-provider grpc server
func NewMocKCSIDriverProviderClient(socketPath string) (*MockCSIDriverProviderClient, error) {
	s := &MockCSIDriverProviderClient{
		socketPath: socketPath,
	}
	return s, nil
}

type MockPluginClientBuilder struct {
	clients     map[string]v1alpha1.CSIDriverProviderClient
	socketPaths []string
	lock        sync.RWMutex
}

func NewPluginClientBuilder(paths []string) *MockPluginClientBuilder {
	pcb := &MockPluginClientBuilder{
		clients:     make(map[string]v1alpha1.CSIDriverProviderClient),
		socketPaths: paths,
		lock:        sync.RWMutex{},
	}
	return pcb
}

// SetReturnError sets expected error
func (m *MockCSIDriverProviderClient) SetReturnError(err error) {
	m.returnErr = err
}

// SetObjects sets expected objects id and version
func (m *MockCSIDriverProviderClient) SetObjects(objects map[string]string) {
	ov := make([]*v1alpha1.ObjectVersion, 0, len(objects))
	for k, v := range objects {
		ov = append(ov, &v1alpha1.ObjectVersion{Id: k, Version: v})
	}
	m.objects = ov
}

// SetFiles sets provider files to return on Mount
func (m *MockCSIDriverProviderClient) SetFiles(files []*v1alpha1.File) {
	ov := make([]*v1alpha1.File, 0, len(files))
	for _, v := range files {
		ov = append(ov, &v1alpha1.File{
			Path:     v.Path,
			Mode:     v.Mode,
			Contents: v.Contents,
		})
	}
	m.files = ov
}

// SetProviderErrorCode sets provider error code to return
func (m *MockCSIDriverProviderClient) SetProviderErrorCode(errorCode string) {
	m.errorCode = errorCode
}

// Mount implements provider csi-provider method
func (m *MockCSIDriverProviderClient) Mount(_ context.Context, req *v1alpha1.MountRequest, _ ...grpc.CallOption) (*v1alpha1.MountResponse, error) {
	var attrib, secret map[string]string
	var filePermission os.FileMode
	var err error

	if m.returnErr != nil {
		return &v1alpha1.MountResponse{}, m.returnErr
	}
	if err = json.Unmarshal([]byte(req.GetAttributes()), &attrib); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attributes, error: %w", err)
	}
	if err = json.Unmarshal([]byte(req.GetSecrets()), &secret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets, error: %w", err)
	}
	if err = json.Unmarshal([]byte(req.GetPermission()), &filePermission); err != nil {
		return nil, fmt.Errorf("failed to unmarshal file permission, error: %w", err)
	}

	return &v1alpha1.MountResponse{
		ObjectVersion: m.objects,
		Error: &v1alpha1.Error{
			Code: m.errorCode,
		},
		Files: m.files,
	}, nil
}

// Version implements provider csi-provider method
func (m *MockCSIDriverProviderClient) Version(_ context.Context, _ *v1alpha1.VersionRequest, _ ...grpc.CallOption) (*v1alpha1.VersionResponse, error) {
	return &v1alpha1.VersionResponse{
		Version:        "v1alpha1",
		RuntimeName:    "fakeprovider",
		RuntimeVersion: "0.0.10",
	}, nil
}

// Get returns a CSIDriverProviderClient for the provider. If an existing client
// is not found a new one will be created and added to the MockPluginClientBuilder.
func (p *MockPluginClientBuilder) Get(_ context.Context, provider string) (v1alpha1.CSIDriverProviderClient, error) {
	var out v1alpha1.CSIDriverProviderClient

	// load a client,
	p.lock.RLock()
	out, ok := p.clients[provider]
	p.lock.RUnlock()
	if ok {
		return out, nil
	}

	// check all paths
	socketPath := filepath.Join(p.socketPaths[0], provider+".sock")
	if socketPath == "" {
		return nil, fmt.Errorf("%w: provider %q", errors.New("provider not found"), provider)
	}

	out, err := NewMocKCSIDriverProviderClient(socketPath)
	if err != nil {
		return nil, err
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	// retry reading from the map in case a concurrent Get(provider) succeeded
	// and added a connection to the map before p.lock.Lock() was acquired.
	if r, ok := p.clients[provider]; ok {
		out = r
	} else {
		p.clients[provider] = out
	}
	return out, nil
}

func (p *MockPluginClientBuilder) Set(ProviderClient v1alpha1.CSIDriverProviderClient, provider string) {
	p.clients[provider] = ProviderClient
}
