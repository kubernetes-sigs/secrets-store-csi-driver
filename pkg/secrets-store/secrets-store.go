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
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/utils/mount"

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

var (
	vendorVersion = "0.0.16"
)

// GetDriver returns a new secrets store driver
func GetDriver() *SecretsStore {
	return &SecretsStore{}
}

func newNodeServer(d *csicommon.CSIDriver, providerVolumePath, minProviderVersions, grpcSupportedProviders, nodeID string, mounter mount.Interface, client client.Client, statsReporter StatsReporter) (*nodeServer, error) {
	// get a map of provider and compatible version
	minProviderVersionsMap, err := version.GetMinimumProviderVersions(minProviderVersions)
	if err != nil {
		return nil, err
	}
	grpcSupportedProvidersMap := make(map[string]bool)
	for _, provider := range strings.Split(grpcSupportedProviders, ";") {
		if len(provider) != 0 {
			grpcSupportedProvidersMap[provider] = true
		}
	}

	if len(minProviderVersionsMap) == 0 {
		klog.Infof("minimum compatible provider versions not specified with --min-provider-version")
	}
	if len(grpcSupportedProvidersMap) == 0 {
		klog.Infof("grpc supported providers not enabled")
	}
	return &nodeServer{
		DefaultNodeServer:      csicommon.NewDefaultNodeServer(d),
		providerVolumePath:     providerVolumePath,
		minProviderVersions:    minProviderVersionsMap,
		mounter:                mounter,
		reporter:               statsReporter,
		nodeID:                 nodeID,
		client:                 client,
		grpcSupportedProviders: grpcSupportedProvidersMap,
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
func (s *SecretsStore) Run(driverName, nodeID, endpoint, providerVolumePath, minProviderVersions, grpcSupportedProviders string, client client.Client) {
	klog.Infof("Driver: %v ", driverName)
	klog.Infof("Version: %s", vendorVersion)
	klog.Infof("Provider Volume Path: %s", providerVolumePath)
	klog.Infof("Minimum provider versions: %s", minProviderVersions)
	klog.Infof("GRPC supported providers: %s", grpcSupportedProviders)

	// Initialize default library driver
	s.driver = csicommon.NewCSIDriver(driverName, vendorVersion, nodeID)
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

	ns, err := newNodeServer(s.driver, providerVolumePath, minProviderVersions, grpcSupportedProviders, nodeID, mount.New(""), client, NewStatsReporter())
	if err != nil {
		klog.Fatalf("failed to initialize node server, error: %+v", err)
	}
	s.ns = ns
	s.cs = newControllerServer(s.driver)
	s.ids = newIdentityServer(s.driver)

	server := csicommon.NewNonBlockingGRPCServer()
	server.Start(endpoint, s.ids, s.cs, s.ns)
	server.Wait()
}
