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
	coreInformers "k8s.io/client-go/informers/core/v1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/secrets-store-csi-driver/controllers"
)

// Informer holds the shared index informers
type Informer struct {
	NodePublishSecretRefSecret cache.SharedIndexInformer
}

// Lister holds the object lister
type Lister struct {
	NodePublishSecretRefSecret SecretLister
}

// Store for secrets with label 'secrets-store.csi.k8s.io/used'
type Store interface {
	// GetNodePublishSecretRefSecret returns the NodePublishSecretRef secret matching name and namespace
	GetNodePublishSecretRefSecret(name, namespace string) (*v1.Secret, error)
	// Run initializes and runs the informers
	Run(stopCh <-chan struct{}) error
}

type k8sStore struct {
	informers *Informer
	listers   *Lister
}

// New returns store.Store for NodePublishSecretRefSecret
func New(kubeClient kubernetes.Interface, resyncPeriod time.Duration, filteredWatchSecret bool) (Store, error) {
	store := &k8sStore{
		informers: &Informer{},
		listers:   &Lister{},
	}

	store.informers.NodePublishSecretRefSecret = newNodePublishSecretRefSecretInformer(kubeClient, resyncPeriod, filteredWatchSecret)
	store.listers.NodePublishSecretRefSecret.Store = store.informers.NodePublishSecretRefSecret.GetStore()

	return store, nil
}

// Run initiates the sync of the informers and caches
func (s k8sStore) Run(stopCh <-chan struct{}) error {
	return s.informers.run(stopCh)
}

// GetNodePublishSecretRefSecret returns the NodePublishSecretRef secret matching name and namespace
func (s k8sStore) GetNodePublishSecretRefSecret(name, namespace string) (*v1.Secret, error) {
	return s.listers.NodePublishSecretRefSecret.GetWithKey(fmt.Sprintf("%s/%s", namespace, name))
}

func (i *Informer) run(stopCh <-chan struct{}) error {
	go i.NodePublishSecretRefSecret.Run(stopCh)

	synced := []cache.InformerSynced{
		i.NodePublishSecretRefSecret.HasSynced,
	}
	if !cache.WaitForCacheSync(stopCh, synced...) {
		return fmt.Errorf("failed to sync informer caches")
	}
	return nil
}

// newNodePublishSecretRefSecretInformer returns a NodePublishSecretRef informer
func newNodePublishSecretRefSecretInformer(kubeClient kubernetes.Interface, resyncPeriod time.Duration, filteredWatchSecret bool) cache.SharedIndexInformer {
	var tweakListOptionsFunc internalinterfaces.TweakListOptionsFunc
	if filteredWatchSecret {
		tweakListOptionsFunc = usedFilterForSecret()
	}
	return coreInformers.NewFilteredSecretInformer(
		kubeClient,
		v1.NamespaceAll,
		resyncPeriod,
		cache.Indexers{},
		tweakListOptionsFunc,
	)
}

// usedFilterForSecret returns tweak options to filter using used label (secrets-store.csi.k8s.io/used=true).
// this label will need to be configured by user for NodePublishSecretRef secrets.
func usedFilterForSecret() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = fmt.Sprintf("%s=true", controllers.SecretUsedLabel)
	}
}
