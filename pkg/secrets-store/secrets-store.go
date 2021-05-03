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

	"github.com/container-storage-interface/spec/lib/go/csi"
	mount "k8s.io/mount-utils"

	"sigs.k8s.io/controller-runtime/pkg/client"

	csicommon "sigs.k8s.io/secrets-store-csi-driver/pkg/csi-common"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/version"

	"k8s.io/klog/v2"
)

// SecretsStore implements the IdentityServer, ControllerServer and
// NodeServer CSI interfaces.
type SecretsStore struct {
	driver *csicommon.CSIDriver
	ns     *nodeServer
	cs     *controllerServer
	ids    *identityServer
}

// GetDriver returns a new secrets store driver
func GetDriver() *SecretsStore {
	return &SecretsStore{}
}

func newNodeServer(d *csicommon.CSIDriver, providerVolumePath, nodeID string, mounter mount.Interface, providerClients *PluginClientBuilder, client client.Client, statsReporter StatsReporter) (*nodeServer, error) {
	return &nodeServer{
		DefaultNodeServer:  csicommon.NewDefaultNodeServer(d),
		providerVolumePath: providerVolumePath,
		mounter:            mounter,
		reporter:           statsReporter,
		nodeID:             nodeID,
		client:             client,
		providerClients:    providerClients,
	}, nil
}

func newControllerServer(d *csicommon.CSIDriver) *controllerServer {
	return &controllerServer{
		DefaultControllerServer: csicommon.NewDefaultControllerServer(d),
		vols:                    make(map[string]csi.Volume),
	}
}

func newIdentityServer(d *csicommon.CSIDriver) *identityServer {
	return &identityServer{
		DefaultIdentityServer: csicommon.NewDefaultIdentityServer(d),
	}
}

// Run starts the CSI plugin
func (s *SecretsStore) Run(ctx context.Context, driverName, nodeID, endpoint, providerVolumePath string, providerClients *PluginClientBuilder, client client.Client) {
	klog.Infof("Driver: %v ", driverName)
	klog.Infof("Version: %s, BuildTime: %s", version.BuildVersion, version.BuildTime)
	klog.Infof("Provider Volume Path: %s", providerVolumePath)
	klog.Infof("GRPC supported providers will be dynamically created")

	// Initialize default library driver
	s.driver = csicommon.NewCSIDriver(driverName, version.BuildVersion, nodeID)
	if s.driver == nil {
		klog.Fatal("Failed to initialize SecretsStore CSI Driver.")
	}
	s.driver.AddControllerServiceCapabilities(
		[]csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		})
	s.driver.AddVolumeCapabilityAccessModes([]csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
	})

	ns, err := newNodeServer(s.driver, providerVolumePath, nodeID, mount.New(""), providerClients, client, NewStatsReporter())
	if err != nil {
		klog.Fatalf("failed to initialize node server, error: %+v", err)
	}

	s.ns = ns
	s.cs = newControllerServer(s.driver)
	s.ids = newIdentityServer(s.driver)

	server := csicommon.NewNonBlockingGRPCServer()
	server.Start(ctx, endpoint, s.ids, s.cs, s.ns)
	server.Wait()
}
