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

package k8s

import (
	"fmt"
	"time"

	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	coreInformers "k8s.io/client-go/informers/core/v1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/controllers"
	secretsStoreClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
	secretsStoreInformers "sigs.k8s.io/secrets-store-csi-driver/pkg/client/informers/externalversions/apis/v1alpha1"
	secretsStoreInternalInterfaces "sigs.k8s.io/secrets-store-csi-driver/pkg/client/informers/externalversions/internalinterfaces"
)

// Informer holds the shared index informers
type Informer struct {
	Pod                          cache.SharedIndexInformer
	Secret                       cache.SharedIndexInformer
	SecretProviderClass          cache.SharedIndexInformer
	SecretProviderClassPodStatus cache.SharedIndexInformer
}

// Lister holds the object lister
type Lister struct {
	Pod                          PodLister
	Secret                       SecretLister
	SecretProviderClass          SecretProviderClassLister
	SecretProviderClassPodStatus SecretProviderClassPodStatusLister
}

type Store interface {
	// GetPod returns the pod matching name and namespace
	GetPod(name, namespace string) (*v1.Pod, error)
	// GetSecret returns the secret matching name and namespace
	GetSecret(name, namespace string) (*v1.Secret, error)
	// GetSecretProviderClass returns the secret provider class matching name and namespace
	GetSecretProviderClass(name, namespace string) (*v1alpha1.SecretProviderClass, error)
	// GetSecretProviderClassPodStatus returns the secret provider class pod status matching key
	GetSecretProviderClassPodStatus(key string) (*v1alpha1.SecretProviderClassPodStatus, error)
	// ListSecretProviderClassPodStatus returns a list of SecretProviderClassPodStatus
	// that match the label for the node the driver is running on
	ListSecretProviderClassPodStatus() ([]*v1alpha1.SecretProviderClassPodStatus, error)
	// Run initializes and runs the informers
	Run(stopCh <-chan struct{}) error
}

type k8sStore struct {
	informers *Informer
	listers   *Lister
}

func New(kubeClient kubernetes.Interface, crdClient secretsStoreClient.Interface, nodeName string, resyncPeriod time.Duration, filteredWatchSecret bool) (Store, error) {
	store := &k8sStore{
		informers: &Informer{},
		listers:   &Lister{},
	}
	store.informers.Pod = newPodInformer(kubeClient, resyncPeriod, nodeName)
	store.listers.Pod.Store = store.informers.Pod.GetStore()

	store.informers.Secret = newSecretInformer(kubeClient, resyncPeriod, filteredWatchSecret)
	store.listers.Secret.Store = store.informers.Secret.GetStore()

	store.informers.SecretProviderClass = newSPCInformer(crdClient, resyncPeriod)
	store.listers.SecretProviderClass.Store = store.informers.SecretProviderClass.GetStore()

	store.informers.SecretProviderClassPodStatus = newSPCPodStatusInformer(crdClient, resyncPeriod, nodeName)
	store.listers.SecretProviderClassPodStatus.Store = store.informers.SecretProviderClassPodStatus.GetStore()

	return store, nil
}

// Run initiates the sync of the informers and caches
func (s k8sStore) Run(stopCh <-chan struct{}) error {
	return s.informers.run(stopCh)
}

// GetPod returns the pod matching name and namespace
func (s k8sStore) GetPod(name, namespace string) (*v1.Pod, error) {
	return s.listers.Pod.GetWithKey(getStoreKey(name, namespace))
}

// GetSecret returns the secret matching name and namespace
func (s k8sStore) GetSecret(name, namespace string) (*v1.Secret, error) {
	return s.listers.Secret.GetWithKey(getStoreKey(name, namespace))
}

// GetSecretProviderClass returns the secret provider class matching name and namespace
func (s k8sStore) GetSecretProviderClass(name, namespace string) (*v1alpha1.SecretProviderClass, error) {
	return s.listers.SecretProviderClass.GetWithKey(getStoreKey(name, namespace))
}

// ListSecretProviderClassPodStatus returns a list of SecretProviderClassPodStatus
// that match the label for the node the driver is running on
func (s k8sStore) ListSecretProviderClassPodStatus() ([]*v1alpha1.SecretProviderClassPodStatus, error) {
	var secretProviderClassPodStatuses []*v1alpha1.SecretProviderClassPodStatus
	for _, item := range s.listers.SecretProviderClassPodStatus.List() {
		spcps, ok := item.(*v1alpha1.SecretProviderClassPodStatus)
		if !ok {
			return nil, fmt.Errorf("failed to cast %T to %s", item, "secretproviderclasspodstatus")
		}
		secretProviderClassPodStatuses = append(secretProviderClassPodStatuses, spcps)
	}
	return secretProviderClassPodStatuses, nil
}

// GetSecretProviderClassPodStatus returns the secret provider class pod status matching key
func (s k8sStore) GetSecretProviderClassPodStatus(key string) (*v1alpha1.SecretProviderClassPodStatus, error) {
	return s.listers.SecretProviderClassPodStatus.GetWithKey(key)
}

func (i *Informer) run(stopCh <-chan struct{}) error {
	go i.Pod.Run(stopCh)
	go i.Secret.Run(stopCh)
	go i.SecretProviderClass.Run(stopCh)
	go i.SecretProviderClassPodStatus.Run(stopCh)

	synced := []cache.InformerSynced{i.Pod.HasSynced, i.Secret.HasSynced, i.SecretProviderClass.HasSynced, i.SecretProviderClassPodStatus.HasSynced}
	if !cache.WaitForCacheSync(stopCh, synced...) {
		return fmt.Errorf("failed to sync informer caches")
	}
	return nil
}

// newPodInformer returns a pod informer configured to do filtered list watch
// based on the spec.nodeName field
func newPodInformer(kubeClient kubernetes.Interface, resyncPeriod time.Duration, nodeName string) cache.SharedIndexInformer {
	return coreInformers.NewFilteredPodInformer(
		kubeClient,
		v1.NamespaceAll,
		resyncPeriod,
		cache.Indexers{},
		nodeNameFilterForPod(nodeName),
	)
}

// newSecretInformer returns a secret informer
func newSecretInformer(kubeClient kubernetes.Interface, resyncPeriod time.Duration, filteredWatchSecret bool) cache.SharedIndexInformer {
	if filteredWatchSecret {
		return coreInformers.NewFilteredSecretInformer(
			kubeClient,
			v1.NamespaceAll,
			resyncPeriod,
			cache.Indexers{},
			managedFilterForSecret(),
		)
	}
	return coreInformers.NewFilteredSecretInformer(
		kubeClient,
		v1.NamespaceAll,
		resyncPeriod,
		cache.Indexers{},
		nil,
	)
}

// newSPCPodStatusInformer returns a spc pod status informer configured to do filtered list watch
// based on the node name label
func newSPCPodStatusInformer(crdClient secretsStoreClient.Interface, resyncPeriod time.Duration, nodeName string) cache.SharedIndexInformer {
	return secretsStoreInformers.NewFilteredSecretProviderClassPodStatusInformer(
		crdClient,
		v1.NamespaceAll,
		resyncPeriod,
		cache.Indexers{},
		nodeNameFilterForSPCPodStatus(nodeName),
	)
}

// newSPCInformer returns a secret provider class informer
func newSPCInformer(crdClient secretsStoreClient.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return secretsStoreInformers.NewSecretProviderClassInformer(
		crdClient,
		v1.NamespaceAll,
		resyncPeriod,
		cache.Indexers{},
	)
}

// nodeNameFilterForSPCPodStatus - CRDs do not yet support field selectors. Instead of that we
// apply labels with node name and then later use the NodeNameFilter to tweak
// options to filter using nodename label.
func nodeNameFilterForSPCPodStatus(nodeName string) secretsStoreInternalInterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = fmt.Sprintf("%s=%s", v1alpha1.InternalNodeLabel, nodeName)
	}
}

// nodeNameFilterForPod returns tweak options to filter using nodename label.
func nodeNameFilterForPod(nodeName string) internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
	}
}

// getStoreKey returns key to use for GetByKey from store
func getStoreKey(name, namespace string) string {
	// client-go cache store uses <namespace>/<name> as key
	return fmt.Sprintf("%s/%s", namespace, name)
}

// managedFilterForSecret returns tweak options to filter using managed label.
func managedFilterForSecret() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = fmt.Sprintf("%s=true", controllers.SecretManagedLabel)
	}
}
