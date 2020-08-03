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
	"io"
	"net"

	"google.golang.org/grpc"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

// Strongly typed address
type providerAddr string

// Strongly typed provider name
type csiProviderName string

type csiProviderClientCreator func(addr providerAddr) (
	providerClient v1alpha1.CSIDriverProviderClient,
	closer io.Closer,
	err error,
)

// csiProviderClient encapsulates all csi-provider methods
type csiProviderClient struct {
	providerName             csiProviderName
	addr                     providerAddr
	csiProviderClientCreator csiProviderClientCreator
}

func newProviderClient(providerName csiProviderName, socketPath string) (*csiProviderClient, error) {
	if providerName == "" {
		return nil, fmt.Errorf("provider name is empty")
	}
	return &csiProviderClient{
		providerName:             providerName,
		addr:                     providerAddr(fmt.Sprintf("%s/%s.sock", socketPath, providerName)),
		csiProviderClientCreator: newCSIProviderClient,
	}, nil
}

func newCSIProviderClient(addr providerAddr) (providerClient v1alpha1.CSIDriverProviderClient, closer io.Closer, err error) {
	var conn *grpc.ClientConn
	conn, err = newGrpcConn(addr)
	if err != nil {
		return nil, nil, err
	}

	providerClient = v1alpha1.NewCSIDriverProviderClient(conn)
	return providerClient, conn, nil
}

func newGrpcConn(addr providerAddr) (*grpc.ClientConn, error) {
	network := "unix"
	return grpc.Dial(
		string(addr),
		grpc.WithInsecure(),
		grpc.WithContextDialer(func(ctx context.Context, target string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, target)
		}),
	)
}

func (c *csiProviderClient) MountContent(ctx context.Context, attributes, secrets, targetPath, permission string) (map[string]string, string, error) {
	client, closer, err := c.csiProviderClientCreator(c.addr)
	if err != nil {
		return nil, FailedToCreateProviderGRPCClient, err
	}
	defer closer.Close()

	req := &v1alpha1.MountRequest{
		Attributes: attributes,
		Secrets:    secrets,
		TargetPath: targetPath,
		Permission: permission,
	}

	resp, err := client.Mount(ctx, req)
	if resp != nil && resp.GetError() != nil && len(resp.GetError().Code) > 0 {
		return nil, resp.GetError().Code, fmt.Errorf("mount request failed with provider error code %s, err: %+v", resp.GetError().Code, err)
	}
	if err != nil {
		return nil, GRPCProviderError, err
	}

	ov := resp.GetObjectVersion()
	if ov == nil {
		return nil, GRPCProviderError, errors.New("missing object versions")
	}
	objectVersions := make(map[string]string)
	for _, v := range ov {
		objectVersions[v.Id] = v.Version
	}
	return objectVersions, "", nil
}
