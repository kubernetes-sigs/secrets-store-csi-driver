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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/constants"
	internalerrors "sigs.k8s.io/secrets-store-csi-driver/pkg/errors"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/k8s"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/fileutil"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type nodeServer struct {
	mounter  mount.Interface
	reporter StatsReporter
	nodeID   string
	client   client.Client
	// reader is an instance of mgr.GetAPIReader that is configured to use the API server.
	// This should be used sparingly and only when the client does not fit the use case.
	reader          client.Reader
	providerClients *PluginClientBuilder
	tokenClient     *k8s.TokenClient
}

const (
	// FilePermission is the permission to be used for the staging target path
	FilePermission os.FileMode = 0644

	// CSIPodName is the name of the pod that the mount is created for
	CSIPodName = "csi.storage.k8s.io/pod.name"
	// CSIPodNamespace is the namespace of the pod that the mount is created for
	CSIPodNamespace = "csi.storage.k8s.io/pod.namespace"
	// CSIPodUID is the UID of the pod that the mount is created for
	CSIPodUID = "csi.storage.k8s.io/pod.uid"
	// CSIPodServiceAccountName is the name of the pod service account that the mount is created for
	CSIPodServiceAccountName = "csi.storage.k8s.io/serviceAccount.name"
	// CSIPodServiceAccountTokens is the service account tokens of the pod that the mount is created for
	CSIPodServiceAccountTokens = "csi.storage.k8s.io/serviceAccount.tokens" //nolint

	secretProviderClassField = "secretProviderClass"
)

//gocyclo:ignore
func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (npvr *csi.NodePublishVolumeResponse, err error) {
	startTime := time.Now()
	var parameters map[string]string
	var providerName string
	var podName, podNamespace, podUID, serviceAccountName string
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
				if unmountErr := ns.mounter.Unmount(targetPath); unmountErr != nil {
					klog.ErrorS(unmountErr, "failed to unmounting target path")
				}
			}
			ns.reporter.ReportNodePublishErrorCtMetric(ctx, providerName, errorReason)
			return
		}
		ns.reporter.ReportNodePublishCtMetric(ctx, providerName)
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
	podName = attrib[CSIPodName]
	podNamespace = attrib[CSIPodNamespace]
	podUID = attrib[CSIPodUID]
	serviceAccountName = attrib[CSIPodServiceAccountName]

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

	klog.V(2).InfoS("node publish volume", "target", targetPath, "volumeId", volumeID, "mount flags", mountFlags, "volumeCapabilities", req.VolumeCapability.String())

	gid := constants.NoGID // Group ID to Chown the volume contents to
	mountVol := req.VolumeCapability.GetMount()
	if len(mountVol.GetVolumeMountGroup()) > 0 {
		fsGroupStr := mountVol.GetVolumeMountGroup()
		klog.V(5).Info("fsGroupStr: %v\n", fsGroupStr)
		gid, err = strconv.ParseInt(fsGroupStr, 10, 64)
		klog.V(5).Info("converted gid: %v\n", gid)
		if err != nil {
			klog.ErrorS(err, "failed to mount secrets store object content", "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName}, "FSGroup: ", fsGroupStr)
			return nil, status.Error(codes.InvalidArgument, "Error parsing FSGroup")
		}
	} else {
		klog.V(5).InfoS("mount group not set", "targetPath", targetPath, "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
	}

	if isMockProvider(providerName) {
		// mock provider is used only for running sanity tests against the driver

		// TODO: until requiresRemount (#585) is supported, "mounted" will always be false
		// and this code will always be called
		if !mounted {
			err := ns.mounter.Mount("tmpfs", targetPath, "tmpfs", []string{})

			if err != nil {
				klog.ErrorS(err, "failed to mount", "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
				return nil, err
			}
		}
		klog.Info("skipping calling provider as it's mock")
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
	// send all the volume attributes sent from kubelet to the provider
	for k, v := range attrib {
		parameters[k] = v
	}
	// csi.storage.k8s.io/serviceAccount.tokens is empty for Kubernetes version < 1.20.
	// For 1.20+, if tokenRequests is set in the CSI driver spec, kubelet will generate
	// a token for the pod and send it to the CSI driver.
	// This check is done for backward compatibility to support passing token from driver
	// to provider irrespective of the Kubernetes version. If the token doesn't exist in the
	// volume request context, the CSI driver will generate the token for the configured audience
	// and send it to the provider in the parameters.
	if parameters[CSIPodServiceAccountTokens] == "" {
		// Inject pod service account token into volume attributes
		serviceAccountTokenAttrs, err := ns.tokenClient.PodServiceAccountTokenAttrs(podNamespace, podName, serviceAccountName, types.UID(podUID))
		if err != nil {
			klog.ErrorS(err, "failed to get service account token attrs", "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
			return nil, err
		}
		for k, v := range serviceAccountTokenAttrs {
			parameters[k] = v
		}
	}

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
	permissionStr, err := json.Marshal(FilePermission)
	if err != nil {
		klog.ErrorS(err, "failed to marshal file permission", "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
		return nil, err
	}

	// TODO: until requiresRemount (#585) is supported, "mounted" will always be false
	// and this code will always be called
	if !mounted {
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
	}
	mounted = true

	var objectVersions map[string]string
	if objectVersions, errorReason, err = ns.mountSecretsStoreObjectContent(ctx, providerName, string(parametersStr), string(secretStr), targetPath, string(permissionStr), podName, gid); err != nil {
		klog.ErrorS(err, "failed to mount secrets store object content", "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName})
		return nil, fmt.Errorf("failed to mount secrets store objects for pod %s/%s, err: %w", podNamespace, podName, err)
	}

	// create or update the secret provider class pod status object
	// SPCPS is created the first time after the pod mount is complete. Update is required in scenarios where
	// the pod with same name (pods created by statefulsets) is moved to a different node and the old SPCPS
	// has not yet been garbage collected.
	if err = createOrUpdateSecretProviderClassPodStatus(ctx, ns.client, ns.reader, podName, podNamespace, podUID, secretProviderClass, targetPath, ns.nodeID, true, objectVersions, gid); err != nil {
		return nil, fmt.Errorf("failed to create secret provider class pod status for pod %s/%s, err: %w", podNamespace, podName, err)
	}

	klog.InfoS("node publish volume complete", "targetPath", targetPath, "pod", klog.ObjectRef{Namespace: podNamespace, Name: podName}, "time", time.Since(startTime))
	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (nuvr *csi.NodeUnpublishVolumeResponse, err error) {
	startTime := time.Now()
	defer func() {
		if err != nil {
			ns.reporter.ReportNodeUnPublishErrorCtMetric(ctx)
			return
		}
		ns.reporter.ReportNodeUnPublishCtMetric(ctx)
	}()

	// Check arguments
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path missing in request")
	}
	targetPath := req.GetTargetPath()

	if isMockTargetPath(targetPath) {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	// explicitly remove the contents from the dir to be able to cleanup the target path in
	// case of a failed unpublish
	files, err := filepath.Glob(filepath.Join(targetPath, "*"))
	if err != nil {
		klog.ErrorS(err, "failed to get files from target path", "targetPath", targetPath)
		return nil, status.Error(codes.Internal, err.Error())
	}
	for _, file := range files {
		if err = os.RemoveAll(file); err != nil {
			klog.ErrorS(err, "failed to delete file from target path", "targetPath", targetPath, "file", file)
		}
	}

	err = mount.CleanupMountPoint(targetPath, ns.mounter, false)
	if err != nil && !os.IsNotExist(err) {
		klog.ErrorS(err, "failed to clean and unmount target path", "targetPath", targetPath, "time", time.Since(startTime))
		return nil, status.Error(codes.Internal, err.Error())
	}

	podUID := fileutil.GetPodUIDFromTargetPath(targetPath)
	if podUID != "" {
		// delete service account token from cache as the pod is deleted
		// to ensure the cache isn't growing indefinitely
		ns.tokenClient.DeleteServiceAccountToken(types.UID(podUID))
	}

	klog.InfoS("node unpublish volume complete", "targetPath", targetPath, "time", time.Since(startTime))
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
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in request")
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

func (ns *nodeServer) mountSecretsStoreObjectContent(ctx context.Context, providerName, attributes, secrets, targetPath, permission, podName string, gid int64) (map[string]string, string, error) {
	if len(attributes) == 0 {
		return nil, "", errors.New("missing attributes")
	}
	if len(targetPath) == 0 {
		return nil, "", errors.New("missing target path")
	}
	if len(permission) == 0 {
		return nil, "", errors.New("missing file permissions")
	}

	client, err := ns.providerClients.Get(ctx, providerName)
	if err != nil {
		return nil, "", fmt.Errorf("error connecting to provider %q: %w", providerName, err)
	}

	klog.InfoS("Using gRPC client", "provider", providerName, "pod", podName)

	return MountContent(ctx, client, attributes, secrets, targetPath, permission, nil, gid)
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.Info("node: getting default node info")

	return &csi.NodeGetInfoResponse{
		NodeId: ns.nodeID,
	}, nil
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeExpandVolume is not implemented")
}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	caps := []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
				},
			},
		},
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_VOLUME_MOUNT_GROUP,
				},
			},
		},
	}

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: caps,
	}, nil
}

func (ns *nodeServer) NodeGetVolumeStats(ctx context.Context, in *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
