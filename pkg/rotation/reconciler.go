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

package rotation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	internalerrors "sigs.k8s.io/secrets-store-csi-driver/pkg/errors"

	"k8s.io/client-go/tools/cache"

	"k8s.io/client-go/util/workqueue"

	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/k8s"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/fileutil"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/secretutil"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	secretsstore "sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	secretsStoreClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"

	log "github.com/sirupsen/logrus"
)

const (
	permission os.FileMode = 0644
)

// Reconciler reconciles and rotates contents in the pod
// and Kubernetes secrets periodically
type Reconciler struct {
	store                k8s.Store
	ctrlReaderClient     client.Reader
	ctrlWriterClient     client.Writer
	providerVolumePath   string
	scheme               *runtime.Scheme
	rotationPollInterval time.Duration
	providerClients      map[string]*secretsstore.CSIProviderClient
	queue                workqueue.RateLimitingInterface
	reporter             StatsReporter
}

// NewReconciler returns a new reconciler for rotation
func NewReconciler(s *runtime.Scheme, providerVolumePath, nodeName string, rotationPollInterval time.Duration) (*Reconciler, error) {
	config, err := buildConfig()
	if err != nil {
		return nil, err
	}
	kubeClient := kubernetes.NewForConfigOrDie(config)
	crdClient := secretsStoreClient.NewForConfigOrDie(config)
	c, err := client.New(config, client.Options{Scheme: s, Mapper: nil})
	if err != nil {
		return nil, err
	}
	store, err := k8s.New(kubeClient, crdClient, nodeName, 5*time.Second)
	if err != nil {
		return nil, err
	}

	return &Reconciler{
		store:                store,
		ctrlReaderClient:     c,
		ctrlWriterClient:     c,
		scheme:               s,
		providerVolumePath:   providerVolumePath,
		rotationPollInterval: rotationPollInterval,
		providerClients:      make(map[string]*secretsstore.CSIProviderClient),
		reporter:             newStatsReporter(),
		queue:                workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}, nil
}

// Run starts the rotation reconciler
func (r *Reconciler) Run(stopCh <-chan struct{}) {
	defer r.queue.ShutDown()
	log.Infof("starting rotation reconciler with poll interval: %s", r.rotationPollInterval)

	ticker := time.NewTicker(r.rotationPollInterval)
	defer ticker.Stop()

	if err := r.store.Run(stopCh); err != nil {
		log.Fatalf("failed to run informers for rotation reconciler, err: %+v", err)
	}

	// TODO (aramase) consider adding more workers to process reconcile concurrently
	for i := 0; i < 1; i++ {
		go wait.Until(r.runWorker, time.Second, stopCh)
	}

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			// The spc pod status informer is configured to do a filtered list watch of spc pod statuses
			// labeled for the same node as the driver. LIST will only return the filtered results.
			spcpsList, err := r.store.ListSecretProviderClassPodStatus()
			if err != nil {
				log.Errorf("[rotation] failed to list secret provider class pod status for node, err: %+v", err)
				continue
			}
			for _, spcps := range spcpsList {
				key, err := cache.MetaNamespaceKeyFunc(spcps)
				if err == nil {
					r.queue.Add(key)
				}
			}
		}
	}
}

func (r *Reconciler) reconcile(ctx context.Context, spcps *v1alpha1.SecretProviderClassPodStatus) (err error) {
	begin := time.Now()
	errorReason := internalerrors.FailedToRotate
	// requiresUpdate is set to true when the new object versions differ from the current object versions
	// after the provider mount request is complete
	var requiresUpdate bool
	var providerName string

	defer func() {
		if err != nil {
			r.reporter.reportRotationErrorCtMetric(providerName, errorReason, requiresUpdate)
			return
		}
		r.reporter.reportRotationCtMetric(providerName, requiresUpdate)
		r.reporter.reportRotationDuration(time.Since(begin).Seconds())
	}()

	spcName, spcNamespace := spcps.Status.SecretProviderClassName, spcps.Namespace

	// get the secret provider class the pod status is referencing from informer cache
	spc, err := r.store.GetSecretProviderClass(spcName, spcNamespace)
	if err != nil {
		errorReason = internalerrors.SecretProviderClassNotFound
		return fmt.Errorf("failed to get secret provider class %s/%s, err: %+v", spcNamespace, spcName, err)
	}
	paramsJSON, err := json.Marshal(spc.Spec.Parameters)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters, err: %+v", err)
	}
	permissionJSON, err := json.Marshal(permission)
	if err != nil {
		return fmt.Errorf("failed to marshal permission, err: %+v", err)
	}
	// get pod from informer cache
	podName, podNamespace := spcps.Status.PodName, spcps.Namespace
	pod, err := r.store.GetPod(podName, podNamespace)
	if err != nil {
		errorReason = internalerrors.PodNotFound
		return fmt.Errorf("failed to get pod %s/%s, err: %+v", podNamespace, podName, err)
	}

	// check if the volume pertaining to the current spc is using nodePublishSecretRef for
	// accessing external secrets store
	var nodePublishSecretRef *v1.LocalObjectReference
	for _, vol := range pod.Spec.Volumes {
		if vol.CSI == nil {
			continue
		}
		if vol.CSI.Driver != "secrets-store.csi.k8s.io" {
			continue
		}
		if vol.CSI.VolumeAttributes["secretProviderClass"] != spc.Name {
			continue
		}
		nodePublishSecretRef = vol.CSI.NodePublishSecretRef
		break
	}

	var secretsJSON []byte
	// read the Kubernetes secret referenced in NodePublishSecretRef and marshal it
	// This comprises the secret parameter in the MountRequest to the provider
	if nodePublishSecretRef != nil {
		secretName := nodePublishSecretRef.Name
		secretNamespace := spcps.Namespace

		// read secret from the informer cache
		secret, err := r.store.GetSecret(secretName, secretNamespace)
		if err != nil {
			errorReason = internalerrors.NodePublishSecretRefNotFound
			return fmt.Errorf("failed to get node publish secret %s/%s, err: %+v", secretNamespace, secretName, err)
		}

		nodePublishSecretData := make(map[string]string)
		for k, v := range secret.Data {
			nodePublishSecretData[k] = string(v)
		}
		secretsJSON, err = json.Marshal(nodePublishSecretData)
		if err != nil {
			return fmt.Errorf("failed to marshal node publish secret data, err: %+v", err)
		}
	}

	// generate a map with the current object versions stored in spc pod status
	// the old object versions are passed on to the provider as part of the MountRequest.
	// the provider can use these current object versions to decide if any action is required
	// and if the objects need to be rotated
	oldObjectVersions := make(map[string]string)
	for _, obj := range spcps.Status.Objects {
		oldObjectVersions[obj.ID] = obj.Version
	}

	providerName = string(spc.Spec.Provider)
	providerClient, err := r.getProviderClient(providerName)
	if err != nil {
		errorReason = internalerrors.FailedToCreateProviderGRPCClient
		return fmt.Errorf("failed to create provider client, err: %+v", err)
	}
	newObjectVersions, errorReason, err := providerClient.MountContent(ctx, string(paramsJSON), string(secretsJSON), spcps.Status.TargetPath, string(permissionJSON), oldObjectVersions)
	if err != nil {
		return fmt.Errorf("failed to rotate objects for pod %s/%s, err: %+v", spcps.Namespace, spcps.Status.PodName, err)
	}

	// compare the old object versions and new object versions to check if any of the objects
	// have been updated by the provider
	for k, v := range newObjectVersions {
		version, ok := oldObjectVersions[k]
		if ok && version == v {
			continue
		}
		requiresUpdate = true
		break
	}

	// this loop is executed if there is a difference in the current versions cached in
	// the secret provider class pod status and the new versions returned by the provider.
	// the diff in versions is populated in the secret provider class pod status and if the
	// secret provider class contains secret objects, then the corresponding kubernetes secrets
	// data is updated with the latest versions
	if requiresUpdate {
		var ov []v1alpha1.SecretProviderClassObject
		for k, v := range newObjectVersions {
			ov = append(ov, v1alpha1.SecretProviderClassObject{ID: k, Version: v})
		}
		spcps.Status.Objects = ov

		log.Debugf("updating versions in secret provider class pod status %s/%s", spcps.Namespace, spcps.Name)
		updateFn := func() (bool, error) {
			err = r.updateSecretProviderClassPodStatus(ctx, spcps)
			if err != nil {
				log.Errorf("failed to update latest versions in spc pod status, err: %+v", err)
				return false, nil
			}
			return true, nil
		}

		if err := wait.ExponentialBackoff(wait.Backoff{
			Steps:    5,
			Duration: 1 * time.Millisecond,
			Factor:   1.0,
			Jitter:   0.1,
		}, updateFn); err != nil {
			return fmt.Errorf("failed to update spc pod status, err: %+v", err)
		}

		if len(spc.Spec.SecretObjects) == 0 {
			log.Debugf("spc %s/%s doesn't contain secret objects for pod %s/%s", spcNamespace, spcName, podNamespace, podName)
			return nil
		}
		files, err := fileutil.GetMountedFiles(spcps.Status.TargetPath)
		if err != nil {
			return fmt.Errorf("failed to get mounted files, err: %+v", err)
		}
		for _, secretObj := range spc.Spec.SecretObjects {
			if err = secretutil.ValidateSecretObject(*secretObj); err != nil {
				log.Errorf("failed validation for secret object in spc %s/%s, err: %+v", spcNamespace, spcName, err)
				continue
			}

			secretType := secretutil.GetSecretType(secretObj.Type)
			var datamap map[string][]byte
			if datamap, err = secretutil.GetSecretData(secretObj.Data, secretType, files); err != nil {
				log.Errorf("failed to get data in spc %s/%s for secret %s, err: %+v", spcNamespace, spcName, secretObj.SecretName, err)
				continue
			}

			patchFn := func() (bool, error) {
				// patch secret data with the new contents
				if err := r.patchSecret(ctx, secretObj.SecretName, spcps.Namespace, datamap); err != nil {
					log.Errorf("failed to patch secret %s/%s, err: %+v", spcps.Namespace, secretObj.SecretName, err)
					return false, nil
				}
				return true, nil
			}

			if err := wait.ExponentialBackoff(wait.Backoff{
				Steps:    5,
				Duration: 1 * time.Millisecond,
				Factor:   1.0,
				Jitter:   0.1,
			}, patchFn); err != nil {
				// continue to ensure error in a single secret doesn't block the updates
				// for all other secret objects defined in SPC
				continue
			}
		}
	}

	return nil
}

// updateSecretProviderClassPodStatus updates secret provider class pod status
func (r *Reconciler) updateSecretProviderClassPodStatus(ctx context.Context, spcPodStatus *v1alpha1.SecretProviderClassPodStatus) error {
	// update the secret provider class pod status
	return r.ctrlWriterClient.Update(ctx, spcPodStatus, &client.UpdateOptions{})
}

// patchSecret patches secret with the new data and returns error if any
func (r *Reconciler) patchSecret(ctx context.Context, name, namespace string, data map[string][]byte) error {
	secret := &v1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := r.ctrlReaderClient.Get(ctx, secretKey, secret)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	patch := client.MergeFromWithOptions(secret.DeepCopy(), client.MergeFromWithOptimisticLock{})
	// Patching data replaces values for existing data keys
	// and appends new keys if it doesn't already exist
	secret.Data = data
	return r.ctrlWriterClient.Patch(ctx, secret, patch)
}

// getProviderClient returns the GRPC provider client to use for mount request
func (r *Reconciler) getProviderClient(providerName string) (*secretsstore.CSIProviderClient, error) {
	// check if the provider client already exists
	if providerClient, exists := r.providerClients[providerName]; exists {
		return providerClient, nil
	}
	// create a new client as it doesn't exist in the reconciler cache
	providerClient, err := secretsstore.NewProviderClient(secretsstore.CSIProviderName(providerName), r.providerVolumePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s provider client, err: %+v", providerName, err)
	}
	r.providerClients[providerName] = providerClient
	return providerClient, nil
}

// runWorker runs a thread that process the queue
func (r *Reconciler) runWorker() {
	for r.processNextItem() {

	}
}

// processNextItem picks the next available item in the queue and triggers reconcile
func (r *Reconciler) processNextItem() bool {
	key, quit := r.queue.Get()
	if quit {
		return false
	}
	defer r.queue.Done(key)
	spcps, err := r.store.GetSecretProviderClassPodStatus(key.(string))
	if err != nil {
		log.Errorf("failed to get spc pod status, error: %+v", err)
		r.handleError(err, key)
	}
	if err = r.reconcile(context.Background(), spcps); err != nil {
		log.Errorf("[rotation] failed to reconcile spc %s/%s for pod %s/%s", spcps.Namespace,
			spcps.Status.SecretProviderClassName, spcps.Namespace, spcps.Status.PodName)
	}

	r.handleError(err, key)
	return true
}

// handleError requeue the key after 10s if there is an error while processing
func (r *Reconciler) handleError(err error, key interface{}) {
	if err == nil {
		r.queue.Forget(key)
		return
	}
	r.queue.AddAfter(key, 10*time.Second)
}

// Create the client config. Use kubeconfig if given, otherwise assume in-cluster.
func buildConfig() (*rest.Config, error) {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	return rest.InClusterConfig()
}
