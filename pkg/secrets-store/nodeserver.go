/*
Copyright 2020 The Kubernetes Authors.

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
	"os"
	"os/exec"
	"runtime"

	"github.com/container-storage-interface/spec/lib/go/csi"

	csicommon "sigs.k8s.io/secrets-store-csi-driver/pkg/csi-common"
	version "sigs.k8s.io/secrets-store-csi-driver/pkg/version"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"k8s.io/utils/mount"
)

type nodeServer struct {
	*csicommon.DefaultNodeServer
	providerVolumePath  string
	minProviderVersions map[string]string
	mounter             mount.Interface
	reporter            StatsReporter
	nodeID              string
}

const (
	permission               os.FileMode = 0644
	csipodname                           = "csi.storage.k8s.io/pod.name"
	csipodnamespace                      = "csi.storage.k8s.io/pod.namespace"
	csipoduid                            = "csi.storage.k8s.io/pod.uid"
	providerField                        = "provider"
	parametersField                      = "parameters"
	secretProviderClassField             = "secretProviderClass"
)

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (npvr *csi.NodePublishVolumeResponse, err error) {
	var parameters map[string]string
	var providerName string
	var podName, podNamespace, podUID string
	errorReason := FailedToMount

	defer func() {
		if err != nil {
			ns.reporter.reportNodePublishErrorCtMetric(providerName, errorReason)
			return
		}
		ns.reporter.reportNodePublishCtMetric(providerName)
	}()

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
	volumeID := req.GetVolumeId()
	attrib := req.GetVolumeContext()
	mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()
	secrets := req.GetSecrets()

	mnt, err := ns.ensureMountPoint(targetPath)
	if err != nil {
		errorReason = FailedToEnsureMountPoint
		return nil, status.Errorf(codes.Internal, "Could not mount target %q: %v", targetPath, err)
	}
	if mnt {
		log.Infof("NodePublishVolume: %s is already mounted", targetPath)
		return &csi.NodePublishVolumeResponse{}, nil
	}

	log.Debugf("target %v, volumeId %v, attributes %v, mountflags %v",
		targetPath, volumeID, attrib, mountFlags)

	secretProviderClass := attrib[secretProviderClassField]
	if secretProviderClass == "" {
		return nil, fmt.Errorf("secretProviderClass is not set")
	}

	item, err := getSecretProviderItemByName(ctx, secretProviderClass)
	if err != nil {
		errorReason = SecretProviderClassNotFound
		return nil, err
	}
	provider, err := getStringFromObjectSpec(item.Object, providerField)
	if err != nil {
		return nil, err
	}
	providerName = provider
	parameters, err = getMapFromObjectSpec(item.Object, parametersField)
	if err != nil {
		return nil, err
	}
	parameters[csipodname] = attrib[csipodname]
	parameters[csipodnamespace] = attrib[csipodnamespace]
	parameters[csipoduid] = attrib[csipoduid]
	podName = parameters[csipodname]
	podNamespace = parameters[csipodnamespace]
	podUID = parameters[csipoduid]

	if isMockProvider(providerName) {
		// mock provider is used only for running sanity tests against the driver
		err := ns.mounter.Mount("tmpfs", targetPath, "tmpfs", []string{})
		if err != nil {
			log.Errorf("mount err: %v for pod: %s, ns: %s", err, podUID, podNamespace)
			return nil, err
		}
		log.Infof("skipping calling provider as it's mock")
		return &csi.NodePublishVolumeResponse{}, nil
	}
	// ensure it's read-only
	if !req.GetReadonly() {
		return nil, status.Error(codes.InvalidArgument, "Readonly is not true in request")
	}
	// get provider volume path
	providerVolumePath := ns.providerVolumePath
	if providerVolumePath == "" {
		return nil, fmt.Errorf("Providers volume path not found. Set PROVIDERS_VOLUME_PATH for pod: %s, ns: %s", podUID, podNamespace)
	}

	providerBinary := ns.getProviderPath(runtime.GOOS, providerName)
	if _, err := os.Stat(providerBinary); err != nil {
		errorReason = ProviderBinaryNotFound
		log.Errorf("failed to find provider %s, err: %v for pod: %s, ns: %s", providerName, err, podUID, podNamespace)
		return nil, err
	}

	parametersStr, err := json.Marshal(parameters)
	if err != nil {
		log.Errorf("failed to marshal parameters, err: %v for pod: %s, ns: %s", err, podUID, podNamespace)
		return nil, err
	}
	secretStr, err := json.Marshal(secrets)
	if err != nil {
		log.Errorf("failed to marshal secrets, err: %v for pod: %s, ns: %s", err, podUID, podNamespace)
		return nil, err
	}
	permissionStr, err := json.Marshal(permission)
	if err != nil {
		log.Errorf("failed to marshal file permission, err: %v for pod: %s, ns: %s", err, podUID, podNamespace)
		return nil, err
	}

	// mount before providers can write content to it
	// In linux Mount tmpfs mounts tmpfs to targetPath
	// In windows Mount tmpfs checks if the targetPath exists and if not, will create the target path
	// https://github.com/kubernetes/utils/blob/master/mount/mount_windows.go#L68-L71
	err = ns.mounter.Mount("tmpfs", targetPath, "tmpfs", []string{})
	if err != nil {
		errorReason = FailedToMount
		log.Errorf("mount err: %v for pod: %s, ns: %s", err, podUID, podNamespace)
		return nil, err
	}

	log.Debugf("Calling provider: %s for pod: %s, ns: %s", providerName, podUID, podNamespace)

	// check if minimum compatible provider version with current driver version is set
	// if minimum version is not provided, skip check
	if _, exists := ns.minProviderVersions[providerName]; !exists {
		log.Warningf("minimum compatible %s provider version not set for pod: %s, ns: %s", providerName, podUID, podNamespace)
	} else {
		// check if provider is compatible with driver
		providerCompatible, err := version.IsProviderCompatible(providerBinary, ns.minProviderVersions[providerName])
		if err != nil {
			return nil, err
		}
		if !providerCompatible {
			errorReason = IncompatibleProviderVersion
			return nil, fmt.Errorf("Minimum supported %s provider version with current driver is %s", providerName, ns.minProviderVersions[providerName])
		}
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
		errorReason = ProviderError
		ns.mounter.Unmount(targetPath)
		log.Errorf("error invoking provider, err: %v, output: %v for pod: %s, ns: %s", err, stderr.String(), podUID, podNamespace)
		return nil, fmt.Errorf("error mounting secret %v for pod: %s, ns: %s", stderr.String(), podUID, podNamespace)
	}
	// create the secret provider class pod status object
	if err = createSecretProviderClassPodStatus(ctx, podName, podNamespace, podUID, secretProviderClass, targetPath, ns.nodeID, true); err != nil {
		return nil, fmt.Errorf("failed to create secret provider class pod status, err: %v", err)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (nuvr *csi.NodeUnpublishVolumeResponse, err error) {
	var secretObjects []interface{}
	var podUID string
	syncK8sSecret := false

	defer func() {
		if err != nil {
			ns.reporter.reportNodeUnPublishErrorCtMetric()
			return
		}
		ns.reporter.reportNodeUnPublishCtMetric()
	}()

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	targetPath := req.GetTargetPath()
	volumeID := req.GetVolumeId()
	files, err := getMountedFiles(targetPath)

	if isMockTargetPath(targetPath) {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	podUID = getPodUIDFromTargetPath(targetPath)
	if len(podUID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Cannot get podUID from Target path")
	}
	item, podNS, err := getItemWithPodID(ctx, podUID)
	if err != nil {
		return nil, err
	}
	if item != nil {
		// [optional field]
		secretObjects, syncK8sSecret, err = getSecretObjectsFromSpec(item)
		if err != nil {
			log.Errorf("getSecretObjectsFromSpec err: %v for pod: %s. skipping sync", err, podUID)
			syncK8sSecret = false
		}
	}

	if syncK8sSecret {
		log.Debugf("[NodeUnpublishVolume] syncK8sSecret is enabled for pod: %s", podUID)
		// ensure podNS is valid as it is required for deleting secrets
		if len(podNS) == 0 {
			return nil, status.Error(codes.InvalidArgument, "Invalid podNS from secretproviderclasses obj")
		}
		// removeK8sObjects deletes secrets mapped to each mounted file
		// it should also delete pod info from the secretProviderClass object's byPod status field
		err := removeK8sObjects(ctx, targetPath, podUID, files, secretObjects)
		if err != nil {
			log.Errorf("removeK8sObjects err: %v for pod: %s", err, podUID)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	// remove files
	if runtime.GOOS == "windows" {
		for _, file := range files {
			err = os.RemoveAll(file)
			if err != nil {
				log.Errorf("failed to remove file %s, err: %v for pod: %s", file, err, podUID)
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}
	err = mount.CleanupMountPoint(targetPath, ns.mounter, false)
	if err != nil {
		log.Errorf("error cleaning and unmounting target path %s, err: %v for pod: %s", targetPath, err, podUID)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Debugf("targetPath %s volumeID %s has been unmounted for pod: %s", targetPath, volumeID, podUID)
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
