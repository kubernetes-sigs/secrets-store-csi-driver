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

package keyvault

import (
	//"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	//"runtime"
	//"strings"

	"github.com/ritazh/keyvault-csi-driver/pkg/csi-common"
	"github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/glog"
	"github.com/pkg/errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/util/mount"
	//volumeutil "k8s.io/kubernetes/pkg/volume/util"
)

type nodeServer struct {
	*csicommon.DefaultNodeServer
}
const (
	permission os.FileMode = 0644
)

// type object struct {
// 	objectType   string
// 	name         string
// }

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// Check arguments
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
	}
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	if req.GetVolumeAttributes() == nil || len(req.GetVolumeAttributes()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume attributes missing in request")
	}

	targetPath := req.GetTargetPath()
	notMnt, err := mount.New("").IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	mounter := mount.New("")
	if !notMnt {
		// testing original mount point, make sure the mount link is valid
		if _, err := ioutil.ReadDir(targetPath); err == nil {
			glog.V(2).Infof("keyvault - already mounted to target %s", targetPath)
			return &csi.NodePublishVolumeResponse{}, nil
		}
		// todo: mount link is invalid, now unmount and remount later (built-in functionality)
		glog.Warningf("keyvault - ReadDir %s failed with %v, unmount this directory", targetPath, err)
		if err := mounter.Unmount(targetPath); err != nil {
			glog.Errorf("keyvault - Unmount directory %s failed with %v", targetPath, err)
			return nil, err
		}
		notMnt = true
	}

	fsType := req.GetVolumeCapability().GetMount().GetFsType()

	readOnly := req.GetReadonly()
	volumeID := req.GetVolumeId()
	attrib := req.GetVolumeAttributes()
	mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()

	glog.V(2).Infof("target %v\nfstype %v\n\nreadonly %v\nvolumeId %v\nattributes %v\nmountflags %v\n",
		targetPath, fsType, readOnly, volumeID, attrib, mountFlags)

	// var keyvaultName string
	// var objects []object

	secrets := req.GetNodePublishSecrets()
	glog.V(2).Infof("secret %v\n", secrets)
	// TODO: Get KV
	options := []string{}
	if readOnly {
		options = append(options, "ro")
	}
	// mountOptions := []string{}
	// mountOptions = volumeutil.JoinMountOptions(mountFlags, options)
	// path := "/tmp/" + volumeID
	// if err := mounter.Mount(path, targetPath, "", mountOptions); err != nil {
	// 	glog.Errorf("keyvault-csi-driver NodePublishVolume failed: %v", err)
	// 	return nil, err
	// }

	keyvaultName := attrib["keyvaultname"]
	keyvaultObjectName := attrib["keyvaultobjectname"]
	keyvaultObjectType := attrib["keyvaultobjecttype"]
	keyvaultObjectVersion := attrib["keyvaultobjectversion"]
	usePodIdentity, _ := strconv.ParseBool(attrib["usepodidentity"])
	resourceGroup := attrib["resourcegroup"]
	subscriptionId := attrib["subscriptionid"]
	tenantId := attrib["tenantid"]

	var clientId, clientSecret string

	if !usePodIdentity {
		glog.V(0).Infoln("using pod identity to access keyvault")
		clientId, clientSecret, err = GetCredential(secrets)
		if err != nil {
			return nil, err
		}
	}
	
	content, err := GetKeyVaultObjectContent(ctx, keyvaultName, keyvaultObjectType, keyvaultObjectName, keyvaultObjectVersion, usePodIdentity, resourceGroup, subscriptionId, tenantId, clientId, clientSecret)
	if err != nil {
		return nil, err
	}
	objectContent := []byte(content)
	if err := ioutil.WriteFile(path.Join(targetPath, keyvaultObjectName), objectContent, permission); err != nil {
		return nil, errors.Wrapf(err, "KeyVault failed to write %s at %s", keyvaultObjectName, targetPath)
	}
	glog.V(0).Infof("KeyVault wrote %s at %s",keyvaultObjectName, targetPath)

	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	targetPath := req.GetTargetPath()
	volumeID := req.GetVolumeId()

	// Unmounting the image
	err := mount.New("").Unmount(req.GetTargetPath())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	glog.V(4).Infof("keyvault: volume %s/%s has been unmounted.", targetPath, volumeID)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}
