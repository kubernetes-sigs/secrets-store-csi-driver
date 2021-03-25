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
	"os"
	"regexp"
	"sync"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"

	internalerrors "sigs.k8s.io/secrets-store-csi-driver/pkg/errors"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

// ServiceConfig is used when building CSIDriverProvider clients. The configured
// retry parameters ensures that RPCs will be retried if the underlying
// connection is not ready.
//
// For more details see:
// https://github.com/grpc/grpc/blob/master/doc/service_config.md
const ServiceConfig = `
{
	"methodConfig": [
		{
			"name": [{"service": "v1alpha1.CSIDriverProvider"}],
			"waitForReady": true,
			"retryPolicy": {
				"MaxAttempts": 3,
				"InitialBackoff": "1s",
				"MaxBackoff": "10s",
				"BackoffMultiplier": 1.1,
				"RetryableStatusCodes": [ "UNAVAILABLE" ]
			}
		}
	]
}
`

var (
	// PluginNameRe is the regular expression used to validate plugin names.
	PluginNameRe        = regexp.MustCompile(`^[a-zA-Z0-9_-]{0,30}$`)
	ErrInvalidProvider  = errors.New("invalid provider")
	ErrProviderNotFound = errors.New("provider not found")
)

// PluginClientBuilder builds and stores grpc clients for communicating with
// provider plugins.
type PluginClientBuilder struct {
	clients    map[string]v1alpha1.CSIDriverProviderClient
	conns      map[string]*grpc.ClientConn
	socketPath string
	lock       sync.RWMutex
}

// NewPluginClientBuilder creates a PluginClientBuilder that will connect to
// plugins in the provided absolute path to a folder. Plugin servers must listen
// to the unix domain socket at:
//
// 		<path>/<plugin_name>.sock
//
// where <plugin_name> must match the PluginNameRe regular expression.
func NewPluginClientBuilder(path string) *PluginClientBuilder {
	return &PluginClientBuilder{
		clients:    make(map[string]v1alpha1.CSIDriverProviderClient),
		conns:      make(map[string]*grpc.ClientConn),
		socketPath: path,
		lock:       sync.RWMutex{},
	}
}

// Get returns a CSIDriverProviderClient for the provider. If an existing client
// is not found a new one will be created and added to the PluginClientBuilder.
func (p *PluginClientBuilder) Get(ctx context.Context, provider string) (v1alpha1.CSIDriverProviderClient, error) {
	var out v1alpha1.CSIDriverProviderClient

	// load a client,
	p.lock.RLock()
	out, ok := p.clients[provider]
	p.lock.RUnlock()
	if ok {
		return out, nil
	}

	// client does not exist, create a new one
	if !PluginNameRe.MatchString(provider) {
		return nil, fmt.Errorf("%w: provider %q", ErrInvalidProvider, provider)
	}

	if _, err := os.Stat(fmt.Sprintf("%s/%s.sock", p.socketPath, provider)); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: provider %q", ErrProviderNotFound, provider)
	}

	conn, err := grpc.Dial(
		fmt.Sprintf("%s/%s.sock", p.socketPath, provider),
		grpc.WithInsecure(), // the interface is only secured through filesystem ACLs
		grpc.WithContextDialer(func(ctx context.Context, target string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", target)
		}),
		grpc.WithDefaultServiceConfig(ServiceConfig),
	)
	if err != nil {
		return nil, err
	}
	out = v1alpha1.NewCSIDriverProviderClient(conn)

	p.lock.Lock()
	defer p.lock.Unlock()

	// retry reading from the map in case a concurrent Get(provider) succeeded
	// and added a connection to the map before p.lock.Lock() was acquired.
	if r, ok := p.clients[provider]; ok {
		out = r
	} else {
		p.conns[provider] = conn
		p.clients[provider] = out
	}

	return out, nil
}

// Cleanup closes all underlying connections and removes all clients.
func (p *PluginClientBuilder) Cleanup() {
	p.lock.Lock()
	defer p.lock.Unlock()

	for k := range p.conns {
		if err := p.conns[k].Close(); err != nil {
			klog.ErrorS(err, "error shutting down provider connection", "provider", k)
		}
	}
	p.clients = make(map[string]v1alpha1.CSIDriverProviderClient)
	p.conns = make(map[string]*grpc.ClientConn)
}

// MountContent calls the client's Mount() RPC with helpers to format the
// request and interpret the response.
func MountContent(ctx context.Context, client v1alpha1.CSIDriverProviderClient, attributes, secrets, targetPath, permission string, oldObjectVersions map[string]string) (map[string]string, string, error) {
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

	resp, err := client.Mount(ctx, req)
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
