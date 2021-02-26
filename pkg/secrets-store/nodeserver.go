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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"

	csicommon "sigs.k8s.io/secrets-store-csi-driver/pkg/csi-common"
	internalerrors "sigs.k8s.io/secrets-store-csi-driver/pkg/errors"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/fileutil"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"k8s.io/utils/mount"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type nodeServer struct {
	*csicommon.DefaultNodeServer
	providerVolumePath string
	mounter            mount.Interface
	reporter           StatsReporter
	nodeID             string
	client             client.Client
	providerClients    *PluginClientBuilder
}

const (
	permission os.FileMode = 0644

	csipodname               = "csi.storage.k8s.io/pod.name"
	csipodnamespace          = "csi.storage.k8s.io/pod.namespace"
	csipoduid                = "csi.storage.k8s.io/pod.uid"
	csipodsa                 = "csi.storage.k8s.io/serviceAccount.name"
	secretProviderClassField = "secretProviderClass"
)

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (npvr *csi.NodePublishVolumeResponse, err error) {
	var parameters map[string]string
	var providerName string
	var podName, podNamespace, podUID string
	var targetPath string
	var mounted bool
	errorReason := internalerrors.FailedToMount

	defer func() {
		if err != nil {
			// if there is an error at any stage during node publish volume and if the path
			// has already been mounted, unmount the target path so the next time kubelet calls
			// again for mount, entire node publish volume is retried
			if targetPath != "" && mounted {
				klog.InfoS("unmounting target path as node publish volume failed", "targetPath", targetPath, "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
				ns.mounter.Unmount(targetPath)
			}
			ns.reporter.ReportNodePublishErrorCtMetric(providerName, errorReason)
			return
		}
		ns.reporter.ReportNodePublishCtMetric(providerName)
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

	targetPath = req.GetTargetPath()
	volumeID := req.GetVolumeId()
	attrib := req.GetVolumeContext()
	mountFlags := req.GetVolumeCapability().GetMount().GetMountFlags()
	secrets := req.GetSecrets()

	secretProviderClass := attrib[secretProviderClassField]
	providerName = attrib["providerName"]
	podName = attrib[csipodname]
	podNamespace = attrib[csipodnamespace]
	podUID = attrib[csipoduid]

	mounted, err = ns.ensureMountPoint(targetPath)
	if err != nil {
		// kubelet will not create the CSI NodePublishVolume target directory in 1.20+, in accordance with the CSI specification.
		// CSI driver needs to properly create and process the target path
		if os.IsNotExist(err) {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to create target path %s, err: %v", targetPath, err)
			}
		} else {
			errorReason = internalerrors.FailedToEnsureMountPoint
			return nil, status.Errorf(codes.Internal, "failed to check if target path %s is mount point, err: %v", targetPath, err)
		}
	}
	if mounted {
		klog.InfoS("target path is already mounted", "targetPath", targetPath, "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
		return &csi.NodePublishVolumeResponse{}, nil
	}

	klog.V(2).InfoS("node publish volume", "target", targetPath, "volumeId", volumeID, "attributes", attrib, "mount flags", mountFlags)

	if isMockProvider(providerName) {
		// mock provider is used only for running sanity tests against the driver
		err := ns.mounter.Mount("tmpfs", targetPath, "tmpfs", []string{})
		if err != nil {
			klog.ErrorS(err, "failed to mount", "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
			return nil, err
		}
		klog.Infof("skipping calling provider as it's mock")
		return &csi.NodePublishVolumeResponse{}, nil
	}

	if secretProviderClass == "" {
		return nil, fmt.Errorf("secretProviderClass is not set")
	}

	spc, err := getSecretProviderItem(ctx, ns.client, secretProviderClass, podNamespace)
	if err != nil {
		errorReason = internalerrors.SecretProviderClassNotFound
		return nil, err
	}
	provider, err := getProviderFromSPC(spc)
	if err != nil {
		return nil, err
	}
	providerName = provider
	parameters, err = getParametersFromSPC(spc)
	if err != nil {
		return nil, err
	}
	parameters[csipodname] = attrib[csipodname]
	parameters[csipodnamespace] = attrib[csipodnamespace]
	parameters[csipoduid] = attrib[csipoduid]
	parameters[csipodsa] = attrib[csipodsa]

	// ensure it's read-only
	if !req.GetReadonly() {
		return nil, status.Error(codes.InvalidArgument, "Readonly is not true in request")
	}

	parametersStr, err := json.Marshal(parameters)
	if err != nil {
		klog.ErrorS(err, "failed to marshal parameters", "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
		return nil, err
	}
	secretStr, err := json.Marshal(secrets)
	if err != nil {
		klog.ErrorS(err, "failed to marshal node publish secrets", "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
		return nil, err
	}
	permissionStr, err := json.Marshal(permission)
	if err != nil {
		klog.ErrorS(err, "failed to marshal file permission", "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
		return nil, err
	}

	// mount before providers can write content to it
	// In linux Mount tmpfs mounts tmpfs to targetPath
	// In windows Mount tmpfs checks if the targetPath exists and if not, will create the target path
	// https://github.com/kubernetes/utils/blob/master/mount/mount_windows.go#L68-L71
	err = ns.mounter.Mount("tmpfs", targetPath, "tmpfs", []string{})
	if err != nil {
		errorReason = internalerrors.FailedToMount
		klog.ErrorS(err, "failed to mount", "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
		return nil, err
	}
	mounted = true
	var objectVersions map[string]string
	if objectVersions, errorReason, err = ns.mountSecretsStoreObjectContent(ctx, providerName, string(parametersStr), string(secretStr), targetPath, string(permissionStr), podName); err != nil {
		return nil, fmt.Errorf("failed to mount secrets store objects for pod %s/%s, err: %v", podNamespace, podName, err)
	}

	// create the secret provider class pod status object
	if err = createSecretProviderClassPodStatus(ctx, ns.client, podName, podNamespace, podUID, secretProviderClass, targetPath, ns.nodeID, true, objectVersions); err != nil {
		return nil, fmt.Errorf("failed to create secret provider class pod status for pod %s/%s, err: %v", podNamespace, podName, err)
	}

	klog.InfoS("node publish volume complete", "targetPath", targetPath, "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (nuvr *csi.NodeUnpublishVolumeResponse, err error) {
	defer func() {
		if err != nil {
			ns.reporter.ReportNodeUnPublishErrorCtMetric()
			return
		}
		ns.reporter.ReportNodeUnPublishCtMetric()
	}()

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	targetPath := req.GetTargetPath()
	// Assume no mounted files if GetMountedFiles fails.
	files, _ := fileutil.GetMountedFiles(targetPath)

	if isMockTargetPath(targetPath) {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	// remove files
	if runtime.GOOS == "windows" {
		for _, file := range files {
			err = os.RemoveAll(file)
			if err != nil {
				klog.ErrorS(err, "failed to remove file from target path", "file", file)
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}
	err = mount.CleanupMountPoint(targetPath, ns.mounter, false)
	if err != nil && !os.IsNotExist(err) {
		klog.ErrorS(err, "failed to clean and unmount target path", "targetPath", targetPath)
		return nil, status.Error(codes.Internal, err.Error())
	}

	klog.InfoS("node unpublish volume complete", "targetPath", targetPath)
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

func (ns *nodeServer) mountSecretsStoreObjectContent(ctx context.Context, providerName, attributes, secrets, targetPath, permission, podName string) (map[string]string, string, error) {
	if len(attributes) == 0 {
		return nil, "", errors.New("missing attributes")
	}
	if len(targetPath) == 0 {
		return nil, "", errors.New("missing target path")
	}
	if len(permission) == 0 {
		return nil, "", errors.New("missing file permissions")
	}
	// get provider volume path
	providerVolumePath := ns.providerVolumePath
	if providerVolumePath == "" {
		return nil, "", fmt.Errorf("providers volume path not found. Set PROVIDERS_VOLUME_PATH")
	}

	client, err := ns.providerClients.Get(ctx, providerName)
	if err != nil {
		return nil, "", fmt.Errorf("error connecting to provider %q: %w", providerName, err)
	}

	klog.InfoS("Using grpc client", "provider", providerName, "pod", podName)

	return MountContent(ctx, client, attributes, secrets, targetPath, permission, nil)
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeExpandVolume is not implemented")
}
