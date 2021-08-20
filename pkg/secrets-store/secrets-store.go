/*
Copyright 2018 The Kubernetes Authors.

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

	"sigs.k8s.io/secrets-store-csi-driver/pkg/version"

	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretsStore implements the IdentityServer, ControllerServer and
// NodeServer CSI interfaces.
type SecretsStore struct {
	endpoint string

	ns  *nodeServer
	cs  *controllerServer
	ids *identityServer
}

func NewSecretsStoreDriver(driverName, nodeID, endpoint, providerVolumePath string, providerClients *PluginClientBuilder, client client.Client) *SecretsStore {
	klog.InfoS("Initializing Secrets Store CSI Driver", "driver", driverName, "version", version.BuildVersion, "buildTime", version.BuildTime)

	ns, err := newNodeServer(providerVolumePath, nodeID, mount.New(""), providerClients, client, NewStatsReporter())
	if err != nil {
		klog.Fatalf("failed to initialize node server, error: %+v", err)
	}

	return &SecretsStore{
		endpoint: endpoint,
		ns:       ns,
		cs:       newControllerServer(),
		ids:      newIdentityServer(driverName, version.BuildVersion),
	}
}

func newNodeServer(providerVolumePath, nodeID string, mounter mount.Interface, providerClients *PluginClientBuilder, client client.Client, statsReporter StatsReporter) (*nodeServer, error) {
	return &nodeServer{
		providerVolumePath: providerVolumePath,
		mounter:            mounter,
		reporter:           statsReporter,
		nodeID:             nodeID,
		client:             client,
		providerClients:    providerClients,
	}, nil
}

// Run starts the CSI plugin
func (s *SecretsStore) Run(ctx context.Context) {
	server := NewNonBlockingGRPCServer()
	server.Start(ctx, s.endpoint, s.ids, s.cs, s.ns)
	server.Wait()
}
