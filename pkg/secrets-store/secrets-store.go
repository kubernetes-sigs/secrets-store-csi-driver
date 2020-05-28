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
	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/utils/mount"

	csicommon "sigs.k8s.io/secrets-store-csi-driver/pkg/csi-common"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/metrics"
	version "sigs.k8s.io/secrets-store-csi-driver/pkg/version"

	log "github.com/sirupsen/logrus"
)

// SecretsStore implements the IdentityServer, ControllerServer and
// NodeServer CSI interfaces.
type SecretsStore struct {
	driver *csicommon.CSIDriver
	ns     *nodeServer
	cs     *controllerServer
	ids    *identityServer
}

var (
	vendorVersion = "0.0.11"
)

// GetDriver returns a new secrets store driver
func GetDriver() *SecretsStore {
	return &SecretsStore{}
}

func newNodeServer(d *csicommon.CSIDriver, providerVolumePath, minProviderVersions, nodeID string) (*nodeServer, error) {
	// get a map of provider and compatible version
	minProviderVersionsMap, err := version.GetMinimumProviderVersions(minProviderVersions)
	if err != nil {
		return nil, err
	}
	if len(minProviderVersionsMap) == 0 {
		log.Infof("minimum compatible provider versions not specified with --min-provider-version")
	}
	return &nodeServer{
		DefaultNodeServer:   csicommon.NewDefaultNodeServer(d),
		providerVolumePath:  providerVolumePath,
		minProviderVersions: minProviderVersionsMap,
		mounter:             mount.New(""),
		reporter:            newStatsReporter(),
		nodeID:              nodeID,
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
func (s *SecretsStore) Run(driverName, nodeID, endpoint, providerVolumePath, minProviderVersions string) {
	log.Infof("Driver: %v ", driverName)
	log.Infof("Version: %s", vendorVersion)
	log.Infof("Provider Volume Path: %s", providerVolumePath)
	log.Infof("Minimum provider versions: %s", minProviderVersions)

	// Initialize default library driver
	s.driver = csicommon.NewCSIDriver(driverName, vendorVersion, nodeID)
	if s.driver == nil {
		log.Fatal("Failed to initialize SecretsStore CSI Driver.")
	}
	s.driver.AddControllerServiceCapabilities(
		[]csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		})
	s.driver.AddVolumeCapabilityAccessModes([]csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
	})

	// initialize metrics exporter
	m, err := metrics.NewMetricsExporter()
	if err != nil {
		log.Fatalf("failed to initialize metrics exporter, error: %+v", err)
	}
	defer m.Stop()
	ns, err := newNodeServer(s.driver, providerVolumePath, minProviderVersions, nodeID)
	if err != nil {
		log.Fatalf("failed to initialize node server, error: %+v", err)
	}
	s.ns = ns
	s.cs = newControllerServer(s.driver)
	s.ids = newIdentityServer(s.driver)

	server := csicommon.NewNonBlockingGRPCServer()
	server.Start(endpoint, s.ids, s.cs, s.ns)
	server.Wait()
}
