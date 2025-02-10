/*
Copyright 2022 The Kubernetes Authors.

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

	v1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	storageinformers "k8s.io/client-go/informers/storage/v1"
	"k8s.io/client-go/kubernetes"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
)

type driverClient struct {
	driverName        string
	csiDriverInformer storageinformers.CSIDriverInformer
	csiDriverLister   storagelisters.CSIDriverLister
}

type DriverClient interface {
	Run(stopCh <-chan struct{}) error
	GetDriver() (*v1.CSIDriver, error)
}

// NewDriverClient creates a new DriverClient
// The client will be used to lookup the CSIDriver.
func NewDriverClient(kubeClient kubernetes.Interface, driverName string, resyncPeriod time.Duration) DriverClient {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resyncPeriod,
		kubeinformers.WithTweakListOptions(
			func(options *metav1.ListOptions) {
				options.FieldSelector = fmt.Sprintf("metadata.name=%s", driverName)
			},
		),
	)

	csiDriverInformer := kubeInformerFactory.Storage().V1().CSIDrivers()
	csiDriverLister := csiDriverInformer.Lister()

	return &driverClient{
		driverName:        driverName,
		csiDriverInformer: csiDriverInformer,
		csiDriverLister:   csiDriverLister,
	}
}

// Run initiates the sync of the informers and caches
func (c *driverClient) Run(stopCh <-chan struct{}) error {
	go c.csiDriverInformer.Informer().Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.csiDriverInformer.Informer().HasSynced) {
		return fmt.Errorf("failed to sync informer caches")
	}
	return nil
}

func (c *driverClient) GetDriver() (*v1.CSIDriver, error) {
	return c.csiDriverLister.Get(c.driverName)
}
