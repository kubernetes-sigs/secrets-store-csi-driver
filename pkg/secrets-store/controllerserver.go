/*
Copyright 2019 The Kubernetes Authors.
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
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/container-storage-interface/spec/lib/go/csi"
	csicommon "github.com/deislabs/secrets-store-csi-driver/pkg/csi-common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"golang.org/x/net/context"
)

type controllerServer struct {
	*csicommon.DefaultControllerServer
	mu   sync.Mutex
	vols map[string]csi.Volume
}

var counter uint64

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if err := cs.Driver.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		return nil, err
	}
	if len(req.GetName()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume name is empty")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "volume_capabilities is empty")
	}
	capacityBytes := req.GetCapacityRange().GetRequiredBytes()
	volumeContext := req.GetParameters()
	volName := req.GetName()

	if volumeContext == nil {
		volumeContext = make(map[string]string)
	}
	volumeContext["providerName"] = "mock_provider"

	// check if volume with same name exists
	existingVol, exists := cs.findVolumeByName(volName)
	// if volume exists and capacity is different then error
	if exists && existingVol.CapacityBytes != capacityBytes {
		return nil, status.Error(codes.AlreadyExists, "volume with same name and diff capacity exists")
	}
	volumeID := existingVol.VolumeId
	if !exists {
		volumeID = fmt.Sprintf("%s-%d", req.GetName(), atomic.AddUint64(&counter, 1))
	}
	newVolume := csi.Volume{
		VolumeId:      volumeID,
		CapacityBytes: capacityBytes,
		VolumeContext: volumeContext,
	}

	cs.addVolume(volName, newVolume)
	return &csi.CreateVolumeResponse{Volume: &newVolume}, nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if err := cs.Driver.ValidateControllerServiceRequest(csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		return nil, err
	}
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume id missing in request")
	}
	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume id missing in request")
	}
	if req.GetVolumeCapabilities() == nil {
		return nil, status.Error(codes.InvalidArgument, "volume_capabilities is empty")
	}
	reqVolID := req.GetVolumeId()
	if _, exists := cs.findVolumeByID(reqVolID); exists {
		return &csi.ValidateVolumeCapabilitiesResponse{}, nil
	}
	return nil, status.Error(codes.NotFound, reqVolID)
}

func (cs *controllerServer) findVolumeByName(volName string) (csi.Volume, bool) {
	return cs.findVolume("name", volName)
}

func (cs *controllerServer) findVolumeByID(volID string) (csi.Volume, bool) {
	return cs.findVolume("id", volID)
}

func (cs *controllerServer) addVolume(name string, vol csi.Volume) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.vols[name] = vol
}

func (cs *controllerServer) findVolume(key, nameOrID string) (csi.Volume, bool) {
	return cs.findVolumeInternal(key, nameOrID)
}

func (cs *controllerServer) findVolumeInternal(key, nameOrID string) (csi.Volume, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	switch key {
	case "name":
		vol, ok := cs.vols[nameOrID]
		return vol, ok

	case "id":
		for _, vol := range cs.vols {
			if strings.EqualFold(nameOrID, vol.VolumeId) {
				return vol, true
			}
		}
	}
	return csi.Volume{}, false
}

func isMockProvider(provider string) bool {
	return strings.EqualFold(provider, "mock_provider")
}
