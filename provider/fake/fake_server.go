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
	"fmt"
	"net"

	"google.golang.org/grpc"

	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

type MockCSIProviderServer struct {
	grpcServer *grpc.Server
	listener   net.Listener
	socketPath string
	returnErr  error
	errorCode  string
	objects    []*v1alpha1.ObjectVersion
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
	go m.grpcServer.Serve(m.listener)
	return nil
}

// Mount implements provider csi-provider method
func (m *MockCSIProviderServer) Mount(ctx context.Context, req *v1alpha1.MountRequest) (*v1alpha1.MountResponse, error) {
	if m.returnErr != nil {
		return &v1alpha1.MountResponse{}, m.returnErr
	}
	if len(req.GetAttributes()) == 0 {
		return nil, fmt.Errorf("missing attributes")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, fmt.Errorf("missing target path")
	}
	if len(req.GetPermission()) == 0 {
		return nil, fmt.Errorf("missing permissions")
	}
	return &v1alpha1.MountResponse{
		ObjectVersion: m.objects,
		Error: &v1alpha1.Error{
			Code: m.errorCode,
		},
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
