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
	"os"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/k8s"
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

func NewSecretsStoreDriver(driverName, nodeID, endpoint string,
	providerClients *PluginClientBuilder,
	client client.Client,
	reader client.Reader,
	tokenClient *k8s.TokenClient) *SecretsStore {
	klog.InfoS("Initializing Secrets Store CSI Driver", "driver", driverName, "version", version.BuildVersion, "buildTime", version.BuildTime)

	sr, err := NewStatsReporter()
	if err != nil {
		klog.ErrorS(err, "failed to initialize stats reporter")
		os.Exit(1)
	}
	ns, err := newNodeServer(nodeID, mount.New(""), providerClients, client, reader, sr, tokenClient)
	if err != nil {
		klog.ErrorS(err, "failed to initialize node server")
		os.Exit(1)
	}

	return &SecretsStore{
		endpoint: endpoint,
		ns:       ns,
		cs:       newControllerServer(),
		ids:      newIdentityServer(driverName, version.BuildVersion),
	}
}

func newNodeServer(nodeID string,
	mounter mount.Interface,
	providerClients *PluginClientBuilder,
	client client.Client,
	reader client.Reader,
	statsReporter StatsReporter,
	tokenClient *k8s.TokenClient) (*nodeServer, error) {
	return &nodeServer{
		mounter:         mounter,
		reporter:        statsReporter,
		nodeID:          nodeID,
		client:          client,
		reader:          reader,
		providerClients: providerClients,
		tokenClient:     tokenClient,
	}, nil
}

// Run starts the CSI plugin
func (s *SecretsStore) Run(ctx context.Context) {
	server := NewNonBlockingGRPCServer()
	server.Start(ctx, s.endpoint, s.ids, s.cs, s.ns)
	server.Wait()
}
