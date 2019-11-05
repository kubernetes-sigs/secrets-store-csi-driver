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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/cast"

	csicommon "github.com/deislabs/secrets-store-csi-driver/pkg/csi-common"

	"github.com/golang/glog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"golang.org/x/net/context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/util/mount"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type nodeServer struct {
	*csicommon.DefaultNodeServer
}

const (
	permission os.FileMode = 0644
)

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	glog.V(0).Infof("NodePublishVolume")
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
	if req.GetVolumeContext() == nil || len(req.GetVolumeContext()) == 0 {
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
			glog.V(2).Infof("secrets-store - already mounted to target %s", targetPath)
			return &csi.NodePublishVolumeResponse{}, nil
		}
		// todo: mount link is invalid, now unmount and remount later (built-in functionality)
		glog.Warningf("secrets-store - ReadDir %s failed with %v, unmount this directory", targetPath, err)
		if err := mounter.Unmount(targetPath); err != nil {
			glog.Errorf("secrets-store - Unmount directory %s failed with %v", targetPath, err)
			return nil, err
		}
	}
	volumeID := req.GetVolumeId()
	attrib := req.GetVolumeContext()
	mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()

	glog.V(5).Infof("target %v\nvolumeId %v\nattributes %v\nmountflags %v\n",
		targetPath, volumeID, attrib, mountFlags)

	secrets := req.GetSecrets()

	secretProviderClass := attrib["secretProviderClass"]
	providerName := attrib["providerName"]
	/// TODO: providerName is here for backward compatibility. Will eventually deprecate.
	if secretProviderClass == "" && providerName == "" {
		return nil, fmt.Errorf("secretProviderClass is not set")
	}
	var parameters map[string]string
	/// TODO: This is here for backward compatibility. Will eventually deprecate.
	if providerName != "" {
		parameters = attrib
	} else {
		secretProviderClassGvk := schema.GroupVersionKind{
			Group:   "secrets-store.csi.k8s.com",
			Version: "v1alpha1",
			Kind:    "SecretProviderClassList",
		}
		instanceList := &unstructured.UnstructuredList{}
		instanceList.SetGroupVersionKind(secretProviderClassGvk)
		cfg, err := config.GetConfig()
		if err != nil {
			return nil, err
		}
		c, err := client.New(cfg, client.Options{Scheme: nil, Mapper: nil})
		if err != nil {
			return nil, err
		}
		err = c.List(ctx, instanceList)
		if err != nil {
			return nil, err
		}
		var secretProvideObject map[string]interface{}
		for _, item := range instanceList.Items {
			glog.V(5).Infof("item obj: %v \n", item.Object)
			if item.GetName() == secretProviderClass {
				secretProvideObject = item.Object
				break
			}
		}
		if secretProvideObject == nil {
			return nil, fmt.Errorf("could not find a matching SecretProviderClass object for the secretProviderClass '%s' specified", secretProviderClass)
		}
		providerSpec := secretProvideObject["spec"]
		providerSpecMap, ok := providerSpec.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("could not cast spec as map[string]interface{}")
		}
		providerName, ok = providerSpecMap["provider"].(string)
		if !ok {
			return nil, fmt.Errorf("could not cast provider as string")
		}
		if providerName == "" {
			return nil, fmt.Errorf("providerName is not set")
		}
		parameters, err = cast.ToStringMapStringE(providerSpecMap["parameters"])
		if err != nil {
			return nil, err
		}
		if len(parameters) == 0 {
			return nil, fmt.Errorf("Failed to initialize provider parameters")
		}

		glog.V(5).Infof("got parameters: %v \n", parameters)
		parameters["csi.storage.k8s.io/pod.name"] = attrib["csi.storage.k8s.io/pod.name"]
		parameters["csi.storage.k8s.io/pod.namespace"] = attrib["csi.storage.k8s.io/pod.namespace"]
	}
	// mount before providers can write content to it
	err = mounter.Mount("tmpfs", targetPath, "tmpfs", []string{})
	if err != nil {
		glog.V(0).Infof("mount err: %v", err)
		return nil, err
	}
	if !isMockProvider(providerName) {
		// ensure it's read-only
		if !req.GetReadonly() {
			return nil, status.Error(codes.InvalidArgument, "Readonly is not true in request")
		}
		// get provider volume path
		providerVolumePath := getProvidersVolumePath()
		if providerVolumePath == "" {
			return nil, fmt.Errorf("Providers volume path not found. Set PROVIDERS_VOLUME_PATH")
		}
		if _, err := os.Stat(fmt.Sprintf("%s/%s/provider-%s", providerVolumePath, providerName, providerName)); err != nil {
			glog.Errorf("failed to find provider %s, err: %v", providerName, err)
			return nil, err
		}
		parametersStr, err := json.Marshal(parameters)
		if err != nil {
			glog.V(0).Infof("failed to marshal parameters, err: %v", err)
			return nil, err
		}
		secretStr, err := json.Marshal(secrets)
		if err != nil {
			glog.V(0).Infof("failed to marshal secrets, err: %v", err)
			return nil, err
		}
		permissionStr, err := json.Marshal(permission)
		if err != nil {
			glog.V(0).Infof("failed to marshal file permission, err: %v", err)
			return nil, err
		}

		glog.Infof("Calling provider: %s", providerName)
		glog.Infof("provider command invoked: %s %s %s %s %s %s %s %s %s",
			fmt.Sprintf("%s/%s/provider-%s", providerVolumePath, providerName, providerName),
			"--attributes", "[REDACTED]",
			"--secrets", "[REDACTED]",
			"--targetPath", string(targetPath),
			"--permission", string(permissionStr))

		out, err := exec.Command(
			fmt.Sprintf("%s/%s/provider-%s", providerVolumePath, providerName, providerName),
			"--attributes", string(parametersStr),
			"--secrets", string(secretStr),
			"--targetPath", string(targetPath),
			"--permission", string(permissionStr),
		).Output()

		if err != nil {
			mounter.Unmount(targetPath)
			glog.Errorf("error invoking provider, err: %v, output %v", err, string(out))
			return nil, err
		}
	} else {
		glog.Infof("skipping calling provider as its mock")
	}

	notMnt, err = mount.New("").IsLikelyNotMountPoint(targetPath)
	if err != nil {
		glog.V(0).Infof("Error checking IsLikelyNotMountPoint: %v", err)
	}
	glog.V(5).Infof("after MountSecretsStoreObjectContent, notMnt: %v", notMnt)
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	glog.V(0).Infof("NodeUnpublishVolume")
	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	targetPath := req.GetTargetPath()
	volumeID := req.GetVolumeId()

	// Unmounting the target
	err := mount.New("").Unmount(req.GetTargetPath())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	glog.V(4).Infof("secrets-store: targetPath %s volumeID %s has been unmounted.", targetPath, volumeID)

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	glog.V(0).Infof("NodeStageVolume")
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
	glog.V(0).Infof("NodeUnstageVolume")
	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}
