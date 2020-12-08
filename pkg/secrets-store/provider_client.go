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
	"net"

	"google.golang.org/grpc"

	internalerrors "sigs.k8s.io/secrets-store-csi-driver/pkg/errors"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

// Strongly typed provider name
type CSIProviderName string

// csiProviderClient encapsulates all csi-provider methods
type CSIProviderClient struct {
	providerName CSIProviderName
	client       v1alpha1.CSIDriverProviderClient
	conn         *grpc.ClientConn
}

func NewProviderClient(providerName CSIProviderName, socketPath string) (*CSIProviderClient, error) {
	if providerName == "" {
		return nil, fmt.Errorf("provider name is empty")
	}

	retryPolicy := `{
		"methodConfig": [{
		  "name": [{"service": "v1alpha1.CSIDriverProvider"}],
		  "waitForReady": true,
		  "retryPolicy": {
			  "MaxAttempts": 5,
			  "InitialBackoff": "1s",
			  "MaxBackoff": "10s",
			  "BackoffMultiplier": 1.0,
			  "RetryableStatusCodes": [ "UNAVAILABLE" ]
		  }
		}]}`

	conn, err := grpc.Dial(
		fmt.Sprintf("%s/%s.sock", socketPath, providerName),
		grpc.WithInsecure(),
		grpc.WithContextDialer(func(ctx context.Context, target string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", target)
		}),
		grpc.WithDefaultServiceConfig(retryPolicy),
	)
	if err != nil {
		return nil, err
	}

	return &CSIProviderClient{
		providerName: providerName,
		client:       v1alpha1.NewCSIDriverProviderClient(conn),
		conn:         conn,
	}, nil

}

func (c *CSIProviderClient) MountContent(ctx context.Context, attributes, secrets, targetPath, permission string, oldObjectVersions map[string]string) (map[string]string, string, error) {
	var objVersions []*v1alpha1.ObjectVersion
	for obj, version := range oldObjectVersions {
		objVersions = append(objVersions, &v1alpha1.ObjectVersion{Id: obj, Version: version})
	}

	req := &v1alpha1.MountRequest{
		Attributes:           attributes,
		Secrets:              secrets,
		TargetPath:           targetPath,
		Permission:           permission,
		CurrentObjectVersion: objVersions,
	}

	resp, err := c.client.Mount(ctx, req)
	if err != nil {
		return nil, internalerrors.GRPCProviderError, err
	}
	if resp != nil && resp.GetError() != nil && len(resp.GetError().Code) > 0 {
		return nil, resp.GetError().Code, fmt.Errorf("mount request failed with provider error code %s", resp.GetError().Code)
	}

	ov := resp.GetObjectVersion()
	if ov == nil {
		return nil, internalerrors.GRPCProviderError, errors.New("missing object versions")
	}
	objectVersions := make(map[string]string)
	for _, v := range ov {
		objectVersions[v.Id] = v.Version
	}
	return objectVersions, "", nil
}

func (c *CSIProviderClient) Close() error {
	return c.conn.Close()
}
