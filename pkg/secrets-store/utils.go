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
	"strings"
	"time"

	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/runtimeutil"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/spcpsutil"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ensureMountPoint reports whether the target is mounted and whether
// it has any content. The caller decides whether to mount, populate,
// or short-circuit.
func (ns *nodeServer) ensureMountPoint(target string) (mounted, hasContent bool, err error) {
	notMnt, err := ns.mounter.IsLikelyNotMountPoint(target)
	if err != nil {
		return false, false, err
	}

	if !notMnt {
		entries, readErr := os.ReadDir(target)
		if readErr == nil {
			return true, len(entries) > 0, nil
		}
		// Mount is in a bad state. Unmount and let the caller perform a
		// fresh mount.
		if unmountErr := ns.mounter.Unmount(target); unmountErr != nil {
			klog.ErrorS(unmountErr, "failed to unmount directory", "targetPath", target)
			return true, false, unmountErr
		}
		return false, false, nil
	}

	if runtimeutil.IsRuntimeWindows() {
		// IsLikelyNotMountPoint always returns notMnt=true on Windows;
		// use directory content as the "is mounted" proxy.
		entries, err := os.ReadDir(target)
		if err != nil {
			return false, false, err
		}
		present := len(entries) > 0
		return present, present, nil
	}

	return false, false, nil
}

func (ns *nodeServer) getLastUpdateTime(target string) (time.Time, error) {
	info, err := os.Stat(target)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// getSecretProviderItem returns the secretproviderclass object by name and namespace
func getSecretProviderItem(ctx context.Context, c client.Client, name, namespace string) (*secretsstorev1.SecretProviderClass, error) {
	spc := &secretsstorev1.SecretProviderClass{}
	spcKey := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	if err := c.Get(ctx, spcKey, spc); err != nil {
		return nil, fmt.Errorf("failed to get secretproviderclass %s/%s, error: %w", namespace, name, err)
	}
	return spc, nil
}

// createOrUpdateSecretProviderClassPodStatus creates secret provider class pod status if not exists.
// if the secret provider class pod status already exists, it'll update the status and owner references.
func createOrUpdateSecretProviderClassPodStatus(ctx context.Context, c client.Client, reader client.Reader, podname, namespace, podUID, spcName, targetPath, nodeID string, mounted bool, objects map[string]string) error {
	var o []secretsstorev1.SecretProviderClassObject
	var err error
	spcpsName := podname + "-" + namespace + "-" + spcName

	for k, v := range objects {
		o = append(o, secretsstorev1.SecretProviderClassObject{ID: k, Version: v})
	}
	o = spcpsutil.OrderSecretProviderClassObjectByID(o)

	spcPodStatus := &secretsstorev1.SecretProviderClassPodStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spcpsName,
			Namespace: namespace,
			Labels:    map[string]string{secretsstorev1.InternalNodeLabel: nodeID},
		},
		Status: secretsstorev1.SecretProviderClassPodStatusStatus{
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

	if err = c.Create(ctx, spcPodStatus); err == nil || !apierrors.IsAlreadyExists(err) {
		return err
	}
	klog.InfoS("secret provider class pod status already exists, updating it", "spcps", klog.ObjectRef{Name: spcPodStatus.Name, Namespace: spcPodStatus.Namespace})

	spcps := &secretsstorev1.SecretProviderClassPodStatus{}
	// the secret provider class pod status with the name already exists, update it
	if err = c.Get(ctx, client.ObjectKey{Name: spcpsName, Namespace: namespace}, spcps); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// the secret provider class pod status could be missing in the cache because it was labeled with a different node
		// label, so we need to get it from the API server
		if err = reader.Get(ctx, client.ObjectKey{Name: spcpsName, Namespace: namespace}, spcps); err != nil {
			return err
		}
	}

	// update the labels of the secret provider class pod status to match the node label
	spcps.Labels[secretsstorev1.InternalNodeLabel] = nodeID
	spcps.Status = spcPodStatus.Status
	spcps.OwnerReferences = spcPodStatus.OwnerReferences

	return c.Update(ctx, spcps)
}

// getProviderFromSPC returns the provider as defined in SecretProviderClass
func getProviderFromSPC(spc *secretsstorev1.SecretProviderClass) (string, error) {
	if len(spc.Spec.Provider) == 0 {
		return "", fmt.Errorf("provider not set in %s/%s", spc.Namespace, spc.Name)
	}
	return string(spc.Spec.Provider), nil
}

// getParametersFromSPC returns the parameters map as defined in SecretProviderClass
func getParametersFromSPC(spc *secretsstorev1.SecretProviderClass) (map[string]string, error) {
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
