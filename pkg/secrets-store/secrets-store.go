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
	"github.com/deislabs/secrets-store-csi-driver/pkg/csi-common"
	"github.com/golang/glog"
)

type SecretsStore struct {
	driver *csicommon.CSIDriver
	ns     *nodeServer
}

type secretsStoreVolume struct {
	VolName string `json:"volName"`
	VolID   string `json:"volID"`
	VolSize int64  `json:"volSize"`
	VolPath string `json:"volPath"`
}

var secretsStoreVolumes map[string]secretsStoreVolume

var (
	vendorVersion = "0.0.3"
)

func init() {
	secretsStoreVolumes = map[string]secretsStoreVolume{}
}

func GetDriver() *SecretsStore {
	return &SecretsStore{}
}

func newNodeServer(d *csicommon.CSIDriver) *nodeServer {
	return &nodeServer{
		DefaultNodeServer: csicommon.NewDefaultNodeServer(d),
	}
}

func (s *SecretsStore) Run(driverName, nodeID, endpoint string) {
	glog.Infof("Driver: %v ", driverName)
	glog.Infof("Version: %s", vendorVersion)

	// Initialize default library driver
	s.driver = csicommon.NewCSIDriver(driverName, vendorVersion, nodeID)
	if s.driver == nil {
		glog.Fatalln("Failed to initialize SecretsStore CSI Driver.")
	}
	s.driver.AddControllerServiceCapabilities(
		[]csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_PUBLISH_READONLY,
		})
	s.driver.AddVolumeCapabilityAccessModes([]csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
	})

	s.ns = newNodeServer(s.driver)

	server := csicommon.NewNonBlockingGRPCServer()
	server.Start(endpoint, csicommon.NewDefaultIdentityServer(s.driver), csicommon.NewDefaultControllerServer(s.driver), s.ns)
	server.Wait()
}
