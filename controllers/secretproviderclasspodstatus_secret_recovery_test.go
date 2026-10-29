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
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgofeaturegate "k8s.io/client-go/features"
	clientfeaturestesting "k8s.io/client-go/features/testing"
	"k8s.io/client-go/metadata"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newTestQueue[T comparable]() workqueue.TypedRateLimitingInterface[T] {
	return workqueue.NewTypedRateLimitingQueue(
		workqueue.NewTypedItemExponentialFailureRateLimiter[T](0, 0),
	)
}

type fakeInformer struct {
	toolscache.SharedIndexInformer
	reflectorResourceVersion string
	store                    toolscache.Store
}

func (f fakeInformer) LastSyncResourceVersion() string { return f.reflectorResourceVersion }
func (f fakeInformer) GetStore() toolscache.Store      { return f.store }

func newFakeInformer(reflectorResourceVersion, storedResourceVersion string) fakeInformer {
	store := toolscache.NewStore(toolscache.MetaNamespaceKeyFunc)
	store.Bookmark(storedResourceVersion)
	return fakeInformer{
		reflectorResourceVersion: reflectorResourceVersion,
		store:                    store,
	}
}

type fakeLister struct {
	metadata.ResourceInterface
	resourceVersion string
	calls           *atomic.Int32
	failures        *atomic.Int32
}

func (f fakeLister) List(context.Context, metav1.ListOptions) (*metav1.PartialObjectMetadataList, error) {
	if f.calls != nil {
		f.calls.Add(1)
	}
	if f.failures != nil && f.failures.Add(-1) >= 0 {
		return nil, errors.New("transient API error")
	}
	return &metav1.PartialObjectMetadataList{ListMeta: metav1.ListMeta{ResourceVersion: f.resourceVersion}}, nil
}

// syncCaches seeds the registry with informers and collections at the given
// resource versions.
func syncCaches(registry *InformerRegistry, observed, current string, lister fakeLister) {
	syncCachesWithResourceVersions(registry, observed, observed, current, lister)
}

func syncCachesWithResourceVersions(registry *InformerRegistry, reflected, stored, current string, lister fakeLister) {
	lister.resourceVersion = current
	for _, obj := range []runtime.Object{
		&corev1.Secret{},
		&secretsstorev1.SecretProviderClassPodStatus{},
		&secretsstorev1.SecretProviderClass{},
		&corev1.Pod{},
	} {
		registry.informers.Store(reflect.TypeOf(obj), informerEntry{
			informer: newFakeInformer(reflected, stored),
			lister:   lister,
		})
	}
}

func newSecretRecoveryReconciler(t *testing.T, cachedClient client.Client, scheme *runtime.Scheme, nodeID string) *SecretProviderClassPodStatusReconciler {
	t.Helper()
	clientfeaturestesting.SetFeatureDuringTest(t, clientgofeaturegate.AtomicFIFO, true)
	informers, err := NewInformerRegistry(t.Context(), nil)
	if err != nil {
		t.Fatalf("creating informer registry: %v", err)
	}
	syncCaches(informers, "1000", "1000", fakeLister{})

	reconciler := newReconciler(cachedClient, scheme, nodeID)
	reconciler.secretReader = secretsConfirmingReader{Reader: cachedClient, api: cachedClient}
	reconciler.informers = informers
	return reconciler
}

func TestConfirmingReaderOnlyConfirmsSecretMisses(t *testing.T) {
	scheme, err := setupScheme()
	if err != nil {
		t.Fatal(err)
	}

	cacheError := errors.New("cache error")
	for _, tc := range []struct {
		name           string
		obj            client.Object
		cacheError     error
		wantNotFound   bool
		wantCacheError bool
		wantLiveGets   int32
	}{
		{name: "Secret", obj: &corev1.Secret{}, wantLiveGets: 1},
		{name: "Pod", obj: &corev1.Pod{}, wantNotFound: true},
		{name: "cache error", obj: &corev1.Secret{}, cacheError: cacheError, wantCacheError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := types.NamespacedName{Namespace: "default", Name: "object"}
			cacheClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			var cacheReader client.Reader = cacheClient
			if tc.cacheError != nil {
				cacheReader = interceptor.NewClient(cacheClient, interceptor.Funcs{
					Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
						return tc.cacheError
					},
				})
			}

			liveClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				newSecret(key.Name, key.Namespace, nil, nil),
				newPod(key.Name, key.Namespace, nil),
			).Build()
			var liveGets atomic.Int32
			liveReader := interceptor.NewClient(liveClient, interceptor.Funcs{
				Get: func(ctx context.Context, api client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					liveGets.Add(1)
					return api.Get(ctx, key, obj, opts...)
				},
			})

			err := (secretsConfirmingReader{Reader: cacheReader, api: liveReader}).Get(t.Context(), key, tc.obj)
			switch {
			case tc.wantNotFound && !apierrors.IsNotFound(err):
				t.Errorf("got error %v, want NotFound", err)
			case tc.wantCacheError && !errors.Is(err, cacheError):
				t.Errorf("got error %v, want cache error", err)
			case !tc.wantNotFound && !tc.wantCacheError && err != nil:
				t.Errorf("Get returned error: %v", err)
			}
			if got := liveGets.Load(); got != tc.wantLiveGets {
				t.Errorf("live Get calls = %d, want %d", got, tc.wantLiveGets)
			}
		})
	}
}

func TestRequestsForDeletedSecret(t *testing.T) {
	g := NewWithT(t)
	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	matchingSPC := newSecretProviderClass("matching-spc", "default")
	matchingSPC.Spec.SecretObjects[0].SecretName = " secret1 "
	unrelatedSPC := newSecretProviderClass("unrelated-spc", "default")
	unrelatedSPC.Spec.SecretObjects[0].SecretName = "other-secret"

	matchingStatus := newSecretProviderClassPodStatus("matching", "default", "node1")
	matchingStatus.Status.SecretProviderClassName = matchingSPC.Name
	unrelatedStatus := newSecretProviderClassPodStatus("unrelated", "default", "node1")
	unrelatedStatus.Status.SecretProviderClassName = unrelatedSPC.Name
	staleStatus := newSecretProviderClassPodStatus("stale", "default", "node1")
	staleStatus.Status.SecretProviderClassName = matchingSPC.Name
	staleStatus.Status.PodName = "deleted-pod"
	completedStatus := newSecretProviderClassPodStatus("completed", "default", "node1")
	completedStatus.Status.SecretProviderClassName = matchingSPC.Name
	completedStatus.Status.PodName = "completed-pod"
	otherNodeStatus := newSecretProviderClassPodStatus("other-node", "default", "node2")
	otherNodeStatus.Status.SecretProviderClassName = matchingSPC.Name

	completedPod := newPod(completedStatus.Status.PodName, completedStatus.Namespace, nil)
	completedPod.Status.Phase = corev1.PodSucceeded
	cachedClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		matchingStatus,
		unrelatedStatus,
		staleStatus,
		completedStatus,
		otherNodeStatus,
		matchingSPC,
		unrelatedSPC,
		newPod("pod1", "default", nil),
		completedPod,
	).WithInterceptorFuncs(interceptor.Funcs{
		List: func(ctx context.Context, cached client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if _, ok := list.(*secretsstorev1.SecretProviderClassPodStatusList); !ok {
				return errors.New("only SecretProviderClassPodStatus candidates should be listed")
			}
			return cached.List(ctx, list, opts...)
		},
	}).Build()

	var liveReads atomic.Int32
	reconciler := newSecretRecoveryReconciler(t, cachedClient, scheme, "node1")
	syncCaches(reconciler.informers, "1000", "1000", fakeLister{calls: &liveReads})

	ctx := context.Background()
	g.Expect(reconciler.informers.AreResourcesSynced(ctx,
		&corev1.Secret{},
		&secretsstorev1.SecretProviderClassPodStatus{},
		&secretsstorev1.SecretProviderClass{},
		&corev1.Pod{},
	)).To(Succeed())
	requests, err := reconciler.requestsForDeletedSecretFromSyncedCaches(ctx, types.NamespacedName{Namespace: "default", Name: "secret1"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(requests).To(ConsistOf(reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: matchingStatus.Namespace, Name: matchingStatus.Name},
	}))
	// One resource version probe per consulted type, no matter how many
	// SecretProviderClassPodStatuses are candidates.
	g.Expect(liveReads.Load()).To(Equal(int32(4)))
}

func TestRequestsForDeletedSecretWaitsForCurrentCache(t *testing.T) {
	g := NewWithT(t)
	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	status := newSecretProviderClassPodStatus("status", "default", "node1")
	spc := newSecretProviderClass("spc1", "default")
	pod := newPod(status.Status.PodName, status.Namespace, nil)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(status, spc, pod).Build()
	reconciler := newSecretRecoveryReconciler(t, client, scheme, "node1")
	// Reflector progress is not enough: the barrier waits until the same resource
	// version has been applied to the store.
	syncCachesWithResourceVersions(reconciler.informers, "1000", "1", "1000", fakeLister{})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = reconciler.informers.AreResourcesSynced(ctx,
		&corev1.Secret{},
		&secretsstorev1.SecretProviderClassPodStatus{},
		&secretsstorev1.SecretProviderClass{},
		&corev1.Pod{},
	)
	g.Expect(err).To(MatchError(ContainSubstring("did not apply resource version 1000")))
}

// TestRequestsForDeletedSecretProbesOnceWhileCatchingUp proves the recorded
// resource version is not re-read on every poll, so a busy collection cannot
// outrun the informer that is chasing it.
func TestRequestsForDeletedSecretProbesOnceWhileCatchingUp(t *testing.T) {
	g := NewWithT(t)
	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := newSecretRecoveryReconciler(t, client, scheme, "node1")

	var probes atomic.Int32
	syncCaches(reconciler.informers, "1", "1000", fakeLister{calls: &probes})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err = reconciler.informers.AreResourcesSynced(ctx,
		&corev1.Secret{},
		&secretsstorev1.SecretProviderClassPodStatus{},
		&secretsstorev1.SecretProviderClass{},
		&corev1.Pod{},
	)
	g.Expect(err).To(HaveOccurred())
	g.Expect(probes.Load()).To(Equal(int32(4)))
}

// TestRequestsForDeletedSecretConfirmsCacheMiss proves a Secret that only left
// the informer's label selector is not mistaken for a deleted one.
func TestRequestsForDeletedSecretConfirmsCacheMiss(t *testing.T) {
	g := NewWithT(t)
	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	unlabeled := newSecret("secret1", "default", nil, nil)
	status := newSecretProviderClassPodStatus("status", "default", "node1")
	spc := newSecretProviderClass("spc1", "default")
	pod := newPod(status.Status.PodName, status.Namespace, nil)

	// the filtered informer no longer holds the Secret, but the API server does
	cachedClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(status, spc, pod).Build()
	liveClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(unlabeled).Build()
	reconciler := newSecretRecoveryReconciler(t, cachedClient, scheme, "node1")
	reconciler.secretReader = secretsConfirmingReader{Reader: cachedClient, api: liveClient}

	requests, err := reconciler.requestsForDeletedSecretFromSyncedCaches(context.Background(), client.ObjectKeyFromObject(unlabeled))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(requests).To(BeEmpty())
}

func TestRequestsForDeletedSecretRequeuesRecreatedManagedSecret(t *testing.T) {
	g := NewWithT(t)
	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	recreated := newSecret("secret1", "default", map[string]string{SecretManagedLabel: "true"}, nil)
	status := newSecretProviderClassPodStatus("status", "default", "node1")
	spc := newSecretProviderClass("spc1", "default")
	pod := newPod(status.Status.PodName, status.Namespace, nil)
	cachedClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(recreated, status, spc, pod).Build()
	reconciler := newSecretRecoveryReconciler(t, cachedClient, scheme, "node1")

	requests, err := reconciler.requestsForDeletedSecretFromSyncedCaches(context.Background(), client.ObjectKeyFromObject(recreated))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(requests).To(ConsistOf(reconcile.Request{NamespacedName: client.ObjectKeyFromObject(status)}))
}

func TestDeletedSecretMappingRetriesTransientErrors(t *testing.T) {
	g := NewWithT(t)
	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	spc := newSecretProviderClass("spc1", "default")
	status := newSecretProviderClassPodStatus("status", "default", "node1")
	pod := newPod(status.Status.PodName, status.Namespace, nil)
	cachedClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(spc, status, pod).Build()
	reconciler := newSecretRecoveryReconciler(t, cachedClient, scheme, "node1")

	var probes, failures atomic.Int32
	failures.Store(1)
	syncCaches(reconciler.informers, "1000", "1000", fakeLister{calls: &probes, failures: &failures})

	recoveryQueue := newTestQueue[types.NamespacedName]()
	defer recoveryQueue.ShutDown()
	controllerQueue := newTestQueue[reconcile.Request]()
	defer controllerQueue.ShutDown()
	items := []types.NamespacedName{
		{Namespace: "default", Name: "secret1"},
		{Namespace: "default", Name: "other-secret"},
	}
	for _, item := range items {
		recoveryQueue.Add(item)
	}

	g.Expect(reconciler.processNextSecretRecoveryBatch(context.Background(), recoveryQueue, controllerQueue)).To(BeTrue())
	g.Eventually(recoveryQueue.Len).Should(Equal(len(items)))
	g.Expect(reconciler.processNextSecretRecoveryBatch(context.Background(), recoveryQueue, controllerQueue)).To(BeTrue())
	// Every consulted type is probed on both passes, once for the whole batch.
	g.Expect(probes.Load()).To(Equal(int32(8)))
	for _, item := range items {
		g.Expect(recoveryQueue.NumRequeues(item)).To(BeZero())
	}

	request, shutdown := controllerQueue.Get()
	g.Expect(shutdown).To(BeFalse())
	controllerQueue.Done(request)
	controllerQueue.Forget(request)
	g.Expect(request.NamespacedName).To(Equal(types.NamespacedName{Namespace: "default", Name: status.Name}))
}

func TestDeletedSecretRecoveryBatchesIrrelevantKeys(t *testing.T) {
	g := NewWithT(t)
	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	cachedClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	var liveReads atomic.Int32
	liveClient := interceptor.NewClient(cachedClient, interceptor.Funcs{
		Get: func(ctx context.Context, api client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			liveReads.Add(1)
			return api.Get(ctx, key, obj, opts...)
		},
	})
	reconciler := newSecretRecoveryReconciler(t, cachedClient, scheme, "node1")
	reconciler.secretReader = secretsConfirmingReader{Reader: cachedClient, api: liveClient}

	var probes atomic.Int32
	syncCaches(reconciler.informers, "1000", "1000", fakeLister{calls: &probes})

	recoveryQueue := newTestQueue[types.NamespacedName]()
	defer recoveryQueue.ShutDown()
	controllerQueue := newTestQueue[reconcile.Request]()
	defer controllerQueue.ShutDown()
	for i := range secretRecoveryBatchSize + 1 {
		recoveryQueue.Add(types.NamespacedName{Namespace: "default", Name: fmt.Sprintf("secret-%d", i)})
	}

	g.Expect(reconciler.processNextSecretRecoveryBatch(t.Context(), recoveryQueue, controllerQueue)).To(BeTrue())
	g.Expect(recoveryQueue.Len()).To(Equal(1))
	g.Expect(controllerQueue.Len()).To(BeZero())
	g.Expect(probes.Load()).To(Equal(int32(4)))
	g.Expect(liveReads.Load()).To(BeZero())

	g.Expect(reconciler.processNextSecretRecoveryBatch(t.Context(), recoveryQueue, controllerQueue)).To(BeTrue())
	g.Expect(recoveryQueue.Len()).To(BeZero())
	g.Expect(probes.Load()).To(Equal(int32(8)))
}

func TestSecretRecoverySourceDrainsRegistryQueue(t *testing.T) {
	g := NewWithT(t)
	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	status := newSecretProviderClassPodStatus("status", "default", "node1")
	spc := newSecretProviderClass("spc1", "default")
	pod := newPod(status.Status.PodName, status.Namespace, nil)
	cachedClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(status, spc, pod).Build()
	reconciler := newSecretRecoveryReconciler(t, cachedClient, scheme, "node1")

	controllerQueue := newTestQueue[reconcile.Request]()
	defer controllerQueue.ShutDown()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// The registry owns the queue it created, so its context is what ends recovery.
	informers, err := NewInformerRegistry(ctx, nil)
	g.Expect(err).NotTo(HaveOccurred())
	reconciler.informers = informers
	syncCaches(reconciler.informers, "1000", "1000", fakeLister{})

	g.Expect(reconciler.startSecretRecovery(ctx, controllerQueue)).To(Succeed())

	// A deletion recorded by the registry's informer handler reaches the controller
	// queue as a request for the SPCPS that still declares the Secret.
	reconciler.informers.DeletedSecrets().Add(types.NamespacedName{Namespace: "default", Name: "secret1"})

	g.Eventually(controllerQueue.Len).Should(Equal(1))
	request, shutdown := controllerQueue.Get()
	g.Expect(shutdown).To(BeFalse())
	g.Expect(request.NamespacedName).To(Equal(types.NamespacedName{Namespace: status.Namespace, Name: status.Name}))
	controllerQueue.Done(request)
	controllerQueue.Forget(request)

	cancel()
	g.Eventually(reconciler.informers.DeletedSecrets().ShuttingDown).Should(BeTrue())
}

func TestReconcileWithoutSecretObjectsDoesNotReadSecrets(t *testing.T) {
	g := NewWithT(t)
	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	spc := newSecretProviderClass("spc1", "default")
	spc.Spec.SecretObjects = nil
	status := newSecretProviderClassPodStatus("status", "default", "node1")
	pod := newPod(status.Status.PodName, status.Namespace, nil)
	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(spc, status, pod).Build()
	reconciler := newSecretRecoveryReconciler(t, baseClient, scheme, "node1")
	// A cached Secret read is what makes the manager start the Secret LIST/WATCH,
	// so an installation that syncs nothing must never issue one.
	intercepted := interceptor.NewClient(baseClient, interceptor.Funcs{
		Get: func(ctx context.Context, cached client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Secret); ok {
				return errors.New("Secret informer must not be started without SecretObjects")
			}
			return cached.Get(ctx, key, obj, opts...)
		},
	})
	reconciler.reader = intercepted
	reconciler.secretReader = secretsConfirmingReader{Reader: intercepted, api: intercepted}

	result, err := reconciler.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: status.Namespace, Name: status.Name},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(Equal(ctrl.Result{}))
}

func TestCreateOrUpdateK8sSecretDoesNotAdoptCacheMiss(t *testing.T) {
	g := NewWithT(t)
	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	key := types.NamespacedName{Namespace: "default", Name: "unmanaged"}
	unmanaged := newSecret(key.Name, key.Namespace, nil, nil)
	unmanaged.Data = map[string][]byte{"owner": []byte("administrator")}
	apiClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(unmanaged).Build()
	cachedClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := newSecretRecoveryReconciler(t, cachedClient, scheme, "node1")
	reconciler.writer = apiClient

	err = reconciler.createOrUpdateK8sSecret(t.Context(), key.Name, key.Namespace,
		map[string][]byte{"owner": []byte("driver")},
		map[string]string{SecretManagedLabel: "true"}, nil, corev1.SecretTypeOpaque)
	g.Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())

	got := &corev1.Secret{}
	g.Expect(apiClient.Get(t.Context(), key, got)).To(Succeed())
	g.Expect(got.Data).To(Equal(unmanaged.Data))
	g.Expect(got.OwnerReferences).To(BeEmpty())
}
