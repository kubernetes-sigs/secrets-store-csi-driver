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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/cast"

	csicommon "github.com/deislabs/secrets-store-csi-driver/pkg/csi-common"
	version "github.com/deislabs/secrets-store-csi-driver/pkg/version"

	log "github.com/sirupsen/logrus"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"golang.org/x/net/context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/mount"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type nodeServer struct {
	*csicommon.DefaultNodeServer
	providerVolumePath  string
	minProviderVersions map[string]string
}

const (
	permission os.FileMode = 0644
)

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	log.Info("NodePublishVolume")
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
			log.Infof("secrets-store - already mounted to target %s", targetPath)
			return &csi.NodePublishVolumeResponse{}, nil
		}
		// todo: mount link is invalid, now unmount and remount later (built-in functionality)
		log.Warningf("secrets-store - ReadDir %s failed with %v, unmount this directory", targetPath, err)
		if err := mounter.Unmount(targetPath); err != nil {
			log.Errorf("secrets-store - Unmount directory %s failed with %v", targetPath, err)
			return nil, err
		}
	}
	volumeID := req.GetVolumeId()
	attrib := req.GetVolumeContext()
	mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()

	log.Debugf("target %v, volumeId %v, attributes %v, mountflags %v",
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
			Group:   "secrets-store.csi.x-k8s.io",
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
			log.Debugf("item obj: %v", item.Object)
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

		log.Debugf("got parameters: %v", parameters)
		parameters["csi.storage.k8s.io/pod.name"] = attrib["csi.storage.k8s.io/pod.name"]
		parameters["csi.storage.k8s.io/pod.namespace"] = attrib["csi.storage.k8s.io/pod.namespace"]
	}

	if !isMockProvider(providerName) {
		// ensure it's read-only
		if !req.GetReadonly() {
			return nil, status.Error(codes.InvalidArgument, "Readonly is not true in request")
		}
		// get provider volume path
		providerVolumePath := ns.providerVolumePath
		if providerVolumePath == "" {
			return nil, fmt.Errorf("Providers volume path not found. Set PROVIDERS_VOLUME_PATH")
		}
		if _, err := os.Stat(fmt.Sprintf("%s/%s/provider-%s", providerVolumePath, providerName, providerName)); err != nil {
			log.Errorf("failed to find provider %s, err: %v", providerName, err)
			return nil, err
		}

		parametersStr, err := json.Marshal(parameters)
		if err != nil {
			log.Errorf("failed to marshal parameters, err: %v", err)
			return nil, err
		}
		secretStr, err := json.Marshal(secrets)
		if err != nil {
			log.Errorf("failed to marshal secrets, err: %v", err)
			return nil, err
		}
		permissionStr, err := json.Marshal(permission)
		if err != nil {
			log.Errorf("failed to marshal file permission, err: %v", err)
			return nil, err
		}

		// mount before providers can write content to it
		err = mounter.Mount("tmpfs", targetPath, "tmpfs", []string{})
		if err != nil {
			log.Errorf("mount err: %v", err)
			return nil, err
		}

		// check if minimum compatible provider version with current driver version is set
		if _, exists := ns.minProviderVersions[providerName]; !exists {
			return nil, fmt.Errorf("minimum compatible %s provider version not set", providerName)
		}

		log.Debugf("Calling provider: %s", providerName)
		providerBinary := fmt.Sprintf("%s/%s/provider-%s", providerVolumePath, providerName, providerName)

		// check if provider is compatible with driver
		providerCompatible, err := version.IsProviderCompatible(providerBinary, ns.minProviderVersions[providerName])
		if err != nil {
			return nil, err
		}
		if !providerCompatible {
			return nil, fmt.Errorf("Minimum supported %s provider version with current driver is %s", providerName, ns.minProviderVersions[providerName])
		}

		args := []string{
			"--attributes", string(parametersStr),
			"--secrets", string(secretStr),
			"--targetPath", string(targetPath),
			"--permission", string(permissionStr),
		}

		log.Infof("provider command invoked: %s %s %v", providerBinary,
			"--attributes [REDACTED] --secrets [REDACTED]", args[4:])

		cmd := exec.Command(
			providerBinary,
			args...,
		)

		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		cmd.Stderr, cmd.Stdout = stderr, stdout

		err = cmd.Run()

		log.Infof(string(stdout.String()))
		if err != nil {
			mounter.Unmount(targetPath)
			log.Errorf("error invoking provider, err: %v, output: %v", err, stderr.String())
			return nil, fmt.Errorf("error mounting secret %v", stderr.String())
		}
	} else {
		// mock provider is used only for running sanity tests against the driver
		err = mounter.Mount("tmpfs", targetPath, "tmpfs", []string{})
		if err != nil {
			log.Errorf("mount err: %v", err)
			return nil, err
		}
		log.Infof("skipping calling provider as its mock")
	}

	notMnt, err = mount.New("").IsLikelyNotMountPoint(targetPath)
	if err != nil {
		log.Errorf("Error checking targetPath %s IsLikelyNotMountPoint: %v", targetPath, err)
		return nil, err
	}
	// this means the mount didn't succeed
	if notMnt {
		return nil, fmt.Errorf("after MountSecretsStoreObjectContent, notMnt: %v", notMnt)
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	log.Infof("NodeUnpublishVolume")
	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	targetPath := req.GetTargetPath()
	volumeID := req.GetVolumeId()

	// check if target path is mount point
	notMnt, err := mount.New("").IsLikelyNotMountPoint(targetPath)
	// if not mount point then no need to unmount
	if (err != nil && os.IsNotExist(err)) || notMnt {
		log.Infof("target path %s is not likely a mount point, notMnt %v", targetPath, notMnt)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// Unmounting the target
	err = mount.New("").Unmount(targetPath)
	if err != nil {
		log.Errorf("error unmounting target path %s, err: %v", targetPath, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	log.Debugf("secrets-store: targetPath %s volumeID %s has been unmounted.", targetPath, volumeID)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	log.Infof("NodeStageVolume")
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
	log.Infof("NodeUnstageVolume")
	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetStagingTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}
