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
	"slices"
	"strings"
	"testing"
	"time"

	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	corev1 "k8s.io/api/core/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgofeaturegate "k8s.io/client-go/features"
	clientfeaturestesting "k8s.io/client-go/features/testing"
	metadatafake "k8s.io/client-go/metadata/fake"
	k8stesting "k8s.io/client-go/testing"
	toolscache "k8s.io/client-go/tools/cache"
	cachetesting "k8s.io/client-go/tools/cache/testing"
)

func newTestRegistry(t *testing.T) *InformerRegistry {
	clientfeaturestesting.SetFeatureDuringTest(t, clientgofeaturegate.AtomicFIFO, true)
	registry, err := NewInformerRegistry(t.Context(), metadatafake.NewSimpleMetadataClient(metadatafake.NewTestScheme()))
	if err != nil {
		t.Fatalf("creating informer registry: %v", err)
	}
	return registry
}

func listedNamespaces(registry *InformerRegistry) []string {
	var namespaces []string
	for _, action := range registry.metadata.(*metadatafake.FakeMetadataClient).Actions() {
		namespaces = append(namespaces, action.(k8stesting.ListAction).GetNamespace())
	}
	return namespaces
}

func newRegistryInformer(registry *InformerRegistry, lw toolscache.ListerWatcher, obj apiruntime.Object) toolscache.SharedIndexInformer {
	return registry.NewInformer(lw, obj, 0, toolscache.Indexers{toolscache.NamespaceIndex: toolscache.MetaNamespaceIndexFunc})
}

// pollInterval is short because every condition polled here is already satisfied
// by the time an informer has delivered its event.
const pollInterval = time.Millisecond

func TestRegistryRecordsInformersByType(t *testing.T) {
	registry := newTestRegistry(t)

	source := cachetesting.NewFakeControllerSource()
	defer source.Shutdown()

	secrets := newRegistryInformer(registry, source, &corev1.Secret{})
	statuses := newRegistryInformer(registry, source, &secretsstorev1.SecretProviderClassPodStatus{})

	for _, tc := range []struct {
		obj  apiruntime.Object
		want toolscache.SharedIndexInformer
	}{
		{&corev1.Secret{}, secrets},
		{&secretsstorev1.SecretProviderClassPodStatus{}, statuses},
	} {
		stored, ok := registry.informers.Load(reflect.TypeOf(tc.obj))
		if !ok {
			t.Fatalf("no %T informer was recorded", tc.obj)
		}
		if got := stored.(informerEntry).informer; got != tc.want {
			t.Errorf("recorded %T informer %p, want %p", tc.obj, got, tc.want)
		}
	}
}

func TestRegistryAreResourcesSynced(t *testing.T) {
	registry := newTestRegistry(t)

	source := cachetesting.NewFakeControllerSource()
	defer source.Shutdown()
	source.Add(newPod("pod1", "default", nil))

	informer := newRegistryInformer(registry, source, &corev1.Pod{})
	ctx := t.Context()
	go informer.RunWithContext(ctx)
	if !toolscache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		t.Fatal("Pod informer never synced")
	}

	if err := registry.AreResourcesSynced(ctx, &corev1.Pod{}); err != nil {
		t.Errorf("synced Pod cache reported an error: %v", err)
	}

	err := registry.AreResourcesSynced(ctx, &corev1.Secret{})
	if err == nil || !strings.Contains(err.Error(), "no *v1.Secret informer has been created") {
		t.Errorf("got error %v, want one about a missing Secret informer", err)
	}

	// A probe must never be able to match a real object.
	if got := listedNamespaces(registry); !slices.Equal(got, []string{fakeNamespace}) {
		t.Errorf("listed namespaces %q, want %q", got, []string{fakeNamespace})
	}
}

func TestRegistryPanicsOnUnwatchedResources(t *testing.T) {
	registry := newTestRegistry(t)

	source := cachetesting.NewFakeControllerSource()
	defer source.Shutdown()

	defer func() {
		recovered, ok := recover().(string)
		if !ok || !strings.Contains(recovered, "lister for *v1.Node not found") {
			t.Errorf("recovered %v, want a panic about an unwatched *v1.Node", recovered)
		}
	}()
	newRegistryInformer(registry, source, &corev1.Node{})
}

// TestRegistryEnqueuesDeletedSecretBeforeStart proves the deletion handler is
// attached while the informer is still stopped, so no deletion can slip past it.
func TestRegistryEnqueuesDeletedSecretBeforeStart(t *testing.T) {
	registry := newTestRegistry(t)
	queue := registry.DeletedSecrets()

	source := cachetesting.NewFakeControllerSource()
	defer source.Shutdown()

	deleted := newSecret("deleted", "default", map[string]string{SecretManagedLabel: "true"}, nil)
	deleted.ResourceVersion = ""
	source.Add(deleted.DeepCopy())

	informer := newRegistryInformer(registry, source, &corev1.Secret{})
	// The handler is registered by NewInformer, before the manager ever starts it.
	if informer.HasSynced() {
		t.Fatal("informer started before the deletion handler was registered")
	}

	ctx := t.Context()
	go informer.RunWithContext(ctx)
	if !toolscache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		t.Fatal("Secret informer never synced")
	}

	source.Delete(deleted.DeepCopy())

	if err := wait.PollUntilContextTimeout(ctx, pollInterval, wait.ForeverTestTimeout, true,
		func(context.Context) (bool, error) { return queue.Len() == 1, nil },
	); err != nil {
		t.Fatalf("waiting for the deleted Secret to be queued: %v", err)
	}
	item, shutdown := queue.Get()
	if shutdown {
		t.Fatal("queue shut down before the deleted Secret was read")
	}
	queue.Done(item)
	queue.Forget(item)

	if want := (types.NamespacedName{Namespace: "default", Name: "deleted"}); item != want {
		t.Errorf("queued %s, want %s", item, want)
	}
}

func TestNewInformerRegistryRequiresAtomicFIFO(t *testing.T) {
	clientfeaturestesting.SetFeatureDuringTest(t, clientgofeaturegate.AtomicFIFO, false)
	registry, err := NewInformerRegistry(t.Context(), metadatafake.NewSimpleMetadataClient(metadatafake.NewTestScheme()))
	if registry != nil {
		t.Errorf("got registry %v with AtomicFIFO disabled, want nil", registry)
	}
	if err == nil || !strings.Contains(err.Error(), "AtomicFIFO") {
		t.Errorf("got error %v, want one requiring AtomicFIFO", err)
	}
}

func TestRegistryIgnoresNonSecretInformers(t *testing.T) {
	registry := newTestRegistry(t)
	queue := registry.DeletedSecrets()

	source := cachetesting.NewFakeControllerSource()
	defer source.Shutdown()
	informer := newRegistryInformer(registry, source, &corev1.Pod{})

	ctx := t.Context()
	go informer.RunWithContext(ctx)
	if !toolscache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		t.Fatal("Pod informer never synced")
	}

	pod := newPod("pod1", "default", nil)
	pod.ResourceVersion = ""
	source.Add(pod.DeepCopy())
	if err := wait.PollUntilContextTimeout(ctx, pollInterval, wait.ForeverTestTimeout, true,
		func(context.Context) (bool, error) {
			_, exists, err := informer.GetStore().GetByKey("default/pod1")
			return exists, err
		},
	); err != nil {
		t.Fatalf("waiting for the Pod to be cached: %v", err)
	}
	source.Delete(pod.DeepCopy())

	// The queue is only for Secrets, so this wait is expected to time out.
	err := wait.PollUntilContextTimeout(ctx, pollInterval, 100*time.Millisecond, true,
		func(context.Context) (bool, error) { return queue.Len() > 0, nil },
	)
	if !wait.Interrupted(err) {
		t.Errorf("a Pod deletion was queued as a Secret deletion: %v", err)
	}
}

func TestRegistryDeletionKeys(t *testing.T) {
	registry := newTestRegistry(t)
	queue := registry.DeletedSecrets()

	secret := newSecret("direct", "default", map[string]string{SecretManagedLabel: "true"}, nil)
	secret.Data = map[string][]byte{"value": []byte("sensitive")}
	registry.enqueueDeletedSecret(secret)
	// Only a copied key is retained, so later mutation of the shared object is inert.
	secret.Name = "mutated-after-enqueue"
	secret.Data["value"][0] = 'X'

	registry.enqueueDeletedSecret(toolscache.DeletedFinalStateUnknown{Key: "default/value-tombstone"})
	registry.enqueueDeletedSecret("not an object")

	want := []types.NamespacedName{
		{Namespace: "default", Name: "direct"},
		{Namespace: "default", Name: "value-tombstone"},
	}
	got := make([]types.NamespacedName, 0, len(want))
	for range want {
		item, shutdown := queue.Get()
		if shutdown {
			t.Fatal("queue shut down before every deletion was read")
		}
		queue.Done(item)
		queue.Forget(item)
		got = append(got, item)
	}
	if !slices.Equal(got, want) {
		t.Errorf("queued %v, want %v", got, want)
	}
	if queue.Len() != 0 {
		t.Errorf("queue holds %d extra items, want none", queue.Len())
	}
}

func TestRegistryRetainsDeletionBursts(t *testing.T) {
	registry := newTestRegistry(t)
	queue := registry.DeletedSecrets()

	const deletions = 1025
	for i := range deletions {
		registry.enqueueDeletedSecret(newSecret(fmt.Sprintf("secret-%d", i), "default", nil, nil))
	}
	if queue.Len() != deletions {
		t.Errorf("queue holds %d deletions, want %d", queue.Len(), deletions)
	}
}
