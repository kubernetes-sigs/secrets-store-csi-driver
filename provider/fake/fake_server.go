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

package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"

	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"

	"google.golang.org/grpc"
)

type MockCSIProviderServer struct {
	grpcServer *grpc.Server
	listener   net.Listener
	socketPath string
	returnErr  error
	errorCode  string
	objects    []*v1alpha1.ObjectVersion
	files      []*v1alpha1.File
}

// NewMocKCSIProviderServer returns a mock csi-provider grpc server
func NewMocKCSIProviderServer(socketPath string) (*MockCSIProviderServer, error) {
	server := grpc.NewServer()
	s := &MockCSIProviderServer{
		grpcServer: server,
		socketPath: socketPath,
	}
	v1alpha1.RegisterCSIDriverProviderServer(server, s)
	return s, nil
}

// SetReturnError sets expected error
func (m *MockCSIProviderServer) SetReturnError(err error) {
	m.returnErr = err
}

// SetObjects sets expected objects id and version
func (m *MockCSIProviderServer) SetObjects(objects map[string]string) {
	var ov []*v1alpha1.ObjectVersion
	for k, v := range objects {
		ov = append(ov, &v1alpha1.ObjectVersion{Id: k, Version: v})
	}
	m.objects = ov
}

// SetFiles sets provider files to return on Mount
func (m *MockCSIProviderServer) SetFiles(files []*v1alpha1.File) {
	var ov []*v1alpha1.File
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
func (m *MockCSIProviderServer) SetProviderErrorCode(errorCode string) {
	m.errorCode = errorCode
}

func (m *MockCSIProviderServer) Start() error {
	var err error
	m.listener, err = net.Listen("unix", m.socketPath)
	if err != nil {
		return err
	}
	go func() {
		if err = m.grpcServer.Serve(m.listener); err != nil {
			return
		}
	}()
	return nil
}

func (m *MockCSIProviderServer) Stop() {
	m.grpcServer.GracefulStop()
}

// Mount implements provider csi-provider method
func (m *MockCSIProviderServer) Mount(ctx context.Context, req *v1alpha1.MountRequest) (*v1alpha1.MountResponse, error) {
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
func (m *MockCSIProviderServer) Version(ctx context.Context, req *v1alpha1.VersionRequest) (*v1alpha1.VersionResponse, error) {
	return &v1alpha1.VersionResponse{
		Version:        "v1alpha1",
		RuntimeName:    "fakeprovider",
		RuntimeVersion: "0.0.10",
	}, nil
}
