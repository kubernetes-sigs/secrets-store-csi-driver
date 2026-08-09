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
	"reflect"
	"strings"
	"time"

	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/runtimeutil"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/spcpsutil"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
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

	if runtimeutil.IsRuntimeWindows() {
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
// if the secret provider class pod status already exists but its node label, status or owner references
// are out of date, it'll be updated. If it already matches the desired state, no write is made.
func createOrUpdateSecretProviderClassPodStatus(ctx context.Context, c client.Client, reader client.Reader, podname, namespace, podUID, spcName, targetPath, nodeID string, mounted bool, objects map[string]string) error {
	var o []secretsstorev1.SecretProviderClassObject
	spcpsName := podname + "-" + namespace + "-" + spcName

	for k, v := range objects {
		o = append(o, secretsstorev1.SecretProviderClassObject{ID: k, Version: v})
	}
	o = spcpsutil.OrderSecretProviderClassObjectByID(o)

	desiredStatus := secretsstorev1.SecretProviderClassPodStatusStatus{
		PodName:                 podname,
		TargetPath:              targetPath,
		Mounted:                 mounted,
		SecretProviderClassName: spcName,
		Objects:                 o,
	}
	// Set owner reference to the pod as the mapping between secret provider class pod status and
	// pod is 1 to 1. When pod is deleted, the spc pod status will automatically be garbage collected
	desiredOwnerReferences := []metav1.OwnerReference{
		{
			APIVersion: "v1",
			Kind:       "Pod",
			Name:       podname,
			UID:        types.UID(podUID),
		},
	}

	// AlreadyExists can happen if we lose a create race against another writer, Conflict can happen
	// if we lose an update race. Both are retried with a fresh read of the object.
	retriable := func(err error) bool {
		return apierrors.IsAlreadyExists(err) || apierrors.IsConflict(err)
	}

	return retry.OnError(retry.DefaultBackoff, retriable, func() error {
		spcps := &secretsstorev1.SecretProviderClassPodStatus{}
		err := c.Get(ctx, client.ObjectKey{Name: spcpsName, Namespace: namespace}, spcps)
		if apierrors.IsNotFound(err) {
			// the secret provider class pod status could be missing from the cache because it was
			// labeled with a different node label, so check directly against the API server before
			// concluding that it does not exist
			err = reader.Get(ctx, client.ObjectKey{Name: spcpsName, Namespace: namespace}, spcps)
		}
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			spcPodStatus := &secretsstorev1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spcpsName,
					Namespace: namespace,
					Labels:    map[string]string{secretsstorev1.InternalNodeLabel: nodeID},
				},
				Status: desiredStatus,
			}
			spcPodStatus.SetOwnerReferences(desiredOwnerReferences)
			return c.Create(ctx, spcPodStatus)
		}

		if spcps.Labels[secretsstorev1.InternalNodeLabel] == nodeID &&
			reflect.DeepEqual(spcps.Status, desiredStatus) &&
			reflect.DeepEqual(spcps.OwnerReferences, desiredOwnerReferences) {
			// already up to date, nothing to write
			return nil
		}
		klog.InfoS("secret provider class pod status already exists, updating it", "spcps", klog.ObjectRef{Name: spcps.Name, Namespace: spcps.Namespace})

		if spcps.Labels == nil {
			spcps.Labels = map[string]string{}
		}
		spcps.Labels[secretsstorev1.InternalNodeLabel] = nodeID
		spcps.Status = desiredStatus
		spcps.OwnerReferences = desiredOwnerReferences

		return c.Update(ctx, spcps)
	})
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
