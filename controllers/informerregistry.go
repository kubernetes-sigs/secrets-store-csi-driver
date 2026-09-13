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
	"reflect"
	"sync"
	"time"

	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	"golang.org/x/sync/errgroup"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/resourceversion"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgofeaturegate "k8s.io/client-go/features"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// fakeNamespace cannot name a real namespace, so listing a namespaced resource
// there returns the collection's resource version and no objects at all.
const fakeNamespace = "@fake:sscsi_ns!"

// syncPollInterval only gates an in memory read, so it can be short.
const syncPollInterval = 100 * time.Millisecond

type watchedResource struct {
	resource   schema.GroupVersionResource
	namespaced bool
}

// watchedResources are the API types the driver caches. Hard coded to avoid live discovery calls.
var watchedResources = map[reflect.Type]watchedResource{
	reflect.TypeFor[*corev1.Secret]():                               {corev1.SchemeGroupVersion.WithResource("secrets"), true},
	reflect.TypeFor[*corev1.Pod]():                                  {corev1.SchemeGroupVersion.WithResource("pods"), true},
	reflect.TypeFor[*secretsstorev1.SecretProviderClass]():          {secretsstorev1.SchemeGroupVersion.WithResource("secretproviderclasses"), true},
	reflect.TypeFor[*secretsstorev1.SecretProviderClassPodStatus](): {secretsstorev1.SchemeGroupVersion.WithResource("secretproviderclasspodstatuses"), true},
}

type informerEntry struct {
	informer cache.SharedIndexInformer
	// lister is scoped to fakeNamespace for namespaced resources
	lister metadata.ResourceInterface
}

// InformerRegistry records the informers the manager cache creates so the driver
// can observe managed Kubernetes Secret deletions and cache freshness without
// creating any watch of its own.
type InformerRegistry struct {
	metadata       metadata.Interface
	informers      sync.Map // reflect.Type of the informer's object -> informerEntry
	deletedSecrets workqueue.TypedRateLimitingInterface[types.NamespacedName]
}

func NewInformerRegistry(ctx context.Context, metadataClient metadata.Interface) (*InformerRegistry, error) {
	if !clientgofeaturegate.FeatureGates().Enabled(clientgofeaturegate.AtomicFIFO) {
		return nil, fmt.Errorf("client-go feature gate %q must be enabled", clientgofeaturegate.AtomicFIFO)
	}

	deletedSecrets := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[types.NamespacedName]())
	go func() {
		<-ctx.Done()
		deletedSecrets.ShutDown()
	}()
	return &InformerRegistry{
		metadata:       metadataClient,
		deletedSecrets: deletedSecrets,
	}, nil
}

// NewInformer implements cache.Options.NewInformer. The manager only starts the
// informer after this returns, so registering the deletion handler here means no
// deletion event can be missed.
func (r *InformerRegistry) NewInformer(
	lw cache.ListerWatcher,
	obj apiruntime.Object,
	resync time.Duration,
	indexers cache.Indexers,
) cache.SharedIndexInformer {
	informer := cache.NewSharedIndexInformer(lw, obj, resync, indexers)
	lister := r.resourceVersionLister(obj)

	if lister == nil {
		panic(fmt.Sprintf("lister for %T not found, update watchedResources to include it", obj))
	}

	r.informers.Store(reflect.TypeOf(obj), informerEntry{
		informer: informer,
		lister:   lister,
	})

	if _, ok := obj.(*corev1.Secret); !ok {
		return informer
	}

	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: r.enqueueDeletedSecret,
	}); err != nil {
		panic(err)
	}
	return informer
}

func (r *InformerRegistry) enqueueDeletedSecret(obj any) {
	name, err := cache.DeletionHandlingObjectToName(obj)
	if err != nil {
		klog.ErrorS(err, "failed to get managed Kubernetes Secret key from deletion event")
		return
	}
	r.deletedSecrets.Add(name.AsNamespacedName())
}

// DeletedSecrets returns the queue of managed Kubernetes Secrets observed to be deleted.
func (r *InformerRegistry) DeletedSecrets() workqueue.TypedRateLimitingInterface[types.NamespacedName] {
	return r.deletedSecrets
}

// AreResourcesSynced blocks until every obj's informer store has applied the
// resource version its collection held when this call started. Each collection
// is probed exactly once, so unrelated write traffic cannot keep raising the bar
// a lagging cache is chasing; the caller's context is the only bound needed.
func (r *InformerRegistry) AreResourcesSynced(ctx context.Context, objs ...apiruntime.Object) error {
	group, ctx := errgroup.WithContext(ctx)
	for _, obj := range objs {
		group.Go(func() error { return r.waitForSync(ctx, obj) })
	}
	return group.Wait()
}

func (r *InformerRegistry) waitForSync(ctx context.Context, obj apiruntime.Object) error {
	value, ok := r.informers.Load(reflect.TypeOf(obj))
	if !ok {
		return fmt.Errorf("no %T informer has been created", obj)
	}
	entry := value.(informerEntry)

	list, err := entry.lister.List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("failed to list %T: %w", obj, err)
	}
	current := list.GetResourceVersion()

	if err := wait.PollUntilContextCancel(ctx, syncPollInterval, true, func(context.Context) (bool, error) {
		observed := entry.informer.GetStore().LastStoreSyncResourceVersion()
		if len(observed) == 0 {
			// the informer has yet to complete its initial list
			return false, nil
		}
		cmp, err := resourceversion.CompareResourceVersion(observed, current)
		if err != nil {
			return false, fmt.Errorf("failed to compare %T resource version %q to %q: %w", obj, observed, current, err)
		}
		return cmp >= 0, nil
	}); err != nil {
		return fmt.Errorf("%T cache did not apply resource version %s: %w", obj, current, err)
	}
	return nil
}

// resourceVersionLister scopes namespaced resources to an impossible namespace so
// a probe transfers no objects, exactly as the storage version migrator does.
func (r *InformerRegistry) resourceVersionLister(obj apiruntime.Object) metadata.ResourceInterface {
	watched, ok := watchedResources[reflect.TypeOf(obj)]
	if !ok {
		return nil
	}
	resource := r.metadata.Resource(watched.resource)
	if watched.namespaced {
		return resource.Namespace(fakeNamespace)
	}
	return resource
}
