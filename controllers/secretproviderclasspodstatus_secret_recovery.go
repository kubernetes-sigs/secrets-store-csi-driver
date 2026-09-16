/*
Copyright 2026 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	secretRecoveryBatchDelay = 10 * time.Millisecond
	secretRecoveryBatchSize  = 100
)

// secretsConfirmingReader distinguishes a deleted Secret from one that only stopped
// matching the managed-label cache selector.
type secretsConfirmingReader struct {
	client.Reader               // this is a cached client
	api           client.Reader // this is a live client to confirm the real, live state
}

func (r secretsConfirmingReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	err := r.Reader.Get(ctx, key, obj, opts...)
	if !apierrors.IsNotFound(err) {
		return err
	}
	// we're only interested in secrets, don't do live calls for any other object
	if _, ok := obj.(*corev1.Secret); !ok {
		return err
	}
	return r.api.Get(ctx, key, obj, opts...)
}

func isSecretInSPC(spc *secretsstorev1.SecretProviderClass, secretName string) bool {
	for _, secretObject := range spc.Spec.SecretObjects {
		if secretObject != nil && strings.TrimSpace(secretObject.SecretName) == secretName {
			return true
		}
	}
	return false
}

// startSecretRecovery turns the registry's deleted Secret keys into reconcile
// requests, so recovery reuses the normal Reconcile path.
func (r *SecretProviderClassPodStatusReconciler) startSecretRecovery(
	ctx context.Context,
	controllerQueue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	go r.runSecretRecoveryWorker(ctx, r.informers.DeletedSecrets(), controllerQueue)
	return nil
}

func (r *SecretProviderClassPodStatusReconciler) runSecretRecoveryWorker(
	ctx context.Context,
	recoveryQueue workqueue.TypedRateLimitingInterface[types.NamespacedName],
	controllerQueue workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	for r.processNextSecretRecoveryBatch(ctx, recoveryQueue, controllerQueue) {
	}
}

func (r *SecretProviderClassPodStatusReconciler) processNextSecretRecoveryBatch(
	ctx context.Context,
	recoveryQueue workqueue.TypedRateLimitingInterface[types.NamespacedName],
	controllerQueue workqueue.TypedRateLimitingInterface[reconcile.Request],
) bool {
	if ctx.Err() != nil {
		return false
	}

	item, shutdown := recoveryQueue.Get()
	if shutdown {
		return false
	}
	batch := []types.NamespacedName{item}
	defer func() {
		for _, item := range batch {
			recoveryQueue.Done(item)
		}
	}()

	select {
	case <-ctx.Done():
		return false
	case <-time.After(secretRecoveryBatchDelay):
	}

	// This worker is the queue's only consumer, so no other goroutine can remove
	// an item between this snapshot and Get. Concurrent additions remain queued
	// for this batch or the next one.
	for range min(recoveryQueue.Len(), secretRecoveryBatchSize-1) {
		item, shutdown := recoveryQueue.Get()
		if shutdown {
			break
		}
		batch = append(batch, item)
	}

	if err := r.informers.AreResourcesSynced(ctx,
		&corev1.Secret{},
		&secretsstorev1.SecretProviderClassPodStatus{},
		&secretsstorev1.SecretProviderClass{},
		&corev1.Pod{},
	); err != nil {
		if ctx.Err() != nil {
			return false
		}
		klog.ErrorS(err, "failed to synchronize caches for managed Kubernetes Secret recovery", "secrets", len(batch))
		for _, item := range batch {
			recoveryQueue.AddRateLimited(item)
		}
		return true
	}

	for _, item := range batch {
		requests, err := r.requestsForDeletedSecretFromSyncedCaches(ctx, item)
		if err != nil {
			if ctx.Err() != nil {
				klog.InfoS("context is closed, returning early", "contextError", ctx.Err())
				return false
			}
			klog.ErrorS(err, "failed to process managed Kubernetes Secret recovery", "secret", klog.KRef(item.Namespace, item.Name))
			recoveryQueue.AddRateLimited(item)
			continue
		}

		recoveryQueue.Forget(item)
		for _, request := range requests {
			controllerQueue.Add(request)
		}
	}
	return true
}

func (r *SecretProviderClassPodStatusReconciler) requestsForDeletedSecretFromSyncedCaches(
	ctx context.Context,
	secretKey types.NamespacedName,
) ([]reconcile.Request, error) {
	spcPodStatusList := &secretsstorev1.SecretProviderClassPodStatusList{}
	if err := r.reader.List(ctx, spcPodStatusList, client.InNamespace(secretKey.Namespace), r.ListOptionsLabelSelector()); err != nil {
		return nil, fmt.Errorf("failed to list SecretProviderClassPodStatuses for deleted Kubernetes Secret %s: %w", secretKey, err)
	}

	currentSPCs := make(map[types.NamespacedName]*secretsstorev1.SecretProviderClass)
	requests := make([]reconcile.Request, 0, len(spcPodStatusList.Items))
	//nolint:gocritic // the per iteration copy costs far less than the reads it guards
	for _, spcPodStatus := range spcPodStatusList.Items {
		if !r.processIfBelongsToNode(&spcPodStatus) {
			continue
		}

		spcKey := types.NamespacedName{Namespace: spcPodStatus.Namespace, Name: spcPodStatus.Status.SecretProviderClassName}
		spc, fetched := currentSPCs[spcKey]
		if !fetched {
			var err error
			spc, err = r.getSPCForSecret(ctx, spcKey, secretKey)
			if err != nil {
				return nil, err
			}
			currentSPCs[spcKey] = spc
		}
		if spc == nil {
			continue
		}

		podKey := types.NamespacedName{Namespace: spcPodStatus.Namespace, Name: spcPodStatus.Status.PodName}
		pod := &corev1.Pod{}
		if err := r.reader.Get(ctx, podKey, pod); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("failed to get Pod %s for deleted Kubernetes Secret %s: %w", podKey, secretKey, err)
		}
		if !pod.GetDeletionTimestamp().IsZero() || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}

		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&spcPodStatus)})
	}

	if len(requests) == 0 {
		return nil, nil
	}

	secret := &corev1.Secret{}
	if err := r.secretReader.Get(ctx, secretKey, secret); err == nil {
		if secret.Labels[SecretManagedLabel] != "true" {
			return nil, nil
		}
	} else if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to read Kubernetes Secret %s: %w", secretKey, err)
	}

	klog.InfoS("managed Kubernetes Secret was deleted, enqueueing SecretProviderClassPodStatuses", "secret", klog.KRef(secretKey.Namespace, secretKey.Name), "requests", len(requests))
	return requests, nil
}

func (r *SecretProviderClassPodStatusReconciler) getSPCForSecret(ctx context.Context, spcKey, secretKey types.NamespacedName) (*secretsstorev1.SecretProviderClass, error) {
	spc := &secretsstorev1.SecretProviderClass{}
	if err := r.reader.Get(ctx, spcKey, spc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get SecretProviderClass %s for deleted Kubernetes Secret %s: %w", spcKey, secretKey, err)
	}
	if !isSecretInSPC(spc, secretKey.Name) {
		return nil, nil
	}
	return spc, nil
}
