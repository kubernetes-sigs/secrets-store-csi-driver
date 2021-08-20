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
	"fmt"
	"os"
	"runtime"
	"strings"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ensureMountPoint ensures mount point is valid
func (ns *nodeServer) ensureMountPoint(target string) (bool, error) {
	notMnt, err := ns.mounter.IsLikelyNotMountPoint(target)
	if err != nil {
		return !notMnt, err
	}

	if !notMnt {
		// testing original mount point, make sure the mount link is valid
		_, err := os.ReadDir(target)
		if err == nil {
			klog.InfoS("already mounted to target", "targetPath", target)
			// already mounted
			return !notMnt, nil
		}
		if err := ns.mounter.Unmount(target); err != nil {
			klog.ErrorS(err, "failed to unmount directory", "targetPath", target)
			return !notMnt, err
		}
		notMnt = true
		// remount it in node publish
		return !notMnt, err
	}

	if runtime.GOOS == "windows" {
		// IsLikelyNotMountPoint always returns notMnt=true for windows as the
		// target path is not a soft link to the global mount
		// instead check if the dir exists for windows and if it's not empty
		// If there are contents in the dir, then objects are already mounted
		f, err := os.ReadDir(target)
		if err != nil {
			return !notMnt, err
		}
		if len(f) > 0 {
			notMnt = false
			return !notMnt, err
		}
	}

	return false, nil
}

// getSecretProviderItem returns the secretproviderclass object by name and namespace
func getSecretProviderItem(ctx context.Context, c client.Client, name, namespace string) (*v1alpha1.SecretProviderClass, error) {
	spc := &v1alpha1.SecretProviderClass{}
	spcKey := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	if err := c.Get(ctx, spcKey, spc); err != nil {
		return nil, fmt.Errorf("failed to get secretproviderclass %s/%s, error: %+v", namespace, name, err)
	}
	return spc, nil
}

// createSecretProviderClassPodStatus creates secret provider class pod status
func createSecretProviderClassPodStatus(ctx context.Context, c client.Client, podname, namespace, podUID, spcName, targetPath, nodeID string, mounted bool, objects map[string]string) error {
	var o []v1alpha1.SecretProviderClassObject
	for k, v := range objects {
		o = append(o, v1alpha1.SecretProviderClassObject{ID: k, Version: v})
	}

	spcPodStatus := &v1alpha1.SecretProviderClassPodStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podname + "-" + namespace + "-" + spcName,
			Namespace: namespace,
			Labels:    map[string]string{v1alpha1.InternalNodeLabel: nodeID},
		},
		Status: v1alpha1.SecretProviderClassPodStatusStatus{
			PodName:                 podname,
			TargetPath:              targetPath,
			Mounted:                 mounted,
			SecretProviderClassName: spcName,
			Objects:                 o,
		},
	}
	// Set owner reference to the pod as the mapping between secret provider class pod status and
	// pod is 1 to 1. When pod is deleted, the spc pod status will automatically be garbage collected
	spcPodStatus.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "v1",
			Kind:       "Pod",
			Name:       podname,
			UID:        types.UID(podUID),
		},
	})

	// create the secret provider class pod status
	err := c.Create(ctx, spcPodStatus, &client.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// getProviderFromSPC returns the provider as defined in SecretProviderClass
func getProviderFromSPC(spc *v1alpha1.SecretProviderClass) (string, error) {
	if len(spc.Spec.Provider) == 0 {
		return "", fmt.Errorf("provider not set in %s/%s", spc.Namespace, spc.Name)
	}
	return string(spc.Spec.Provider), nil
}

// getParametersFromSPC returns the parameters map as defined in SecretProviderClass
func getParametersFromSPC(spc *v1alpha1.SecretProviderClass) (map[string]string, error) {
	if len(spc.Spec.Parameters) == 0 {
		return nil, fmt.Errorf("parameters not set in %s/%s", spc.Namespace, spc.Name)
	}
	return spc.Spec.Parameters, nil
}

// isMockProvider returns true if the provider is mock
func isMockProvider(provider string) bool {
	return strings.EqualFold(provider, "mock_provider")
}

// isMockTargetPath returns true if the target path is mock
func isMockTargetPath(targetPath string) bool {
	return strings.EqualFold(targetPath, "/tmp/csi/mount")
}
