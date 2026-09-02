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

package controllers

import (
	"context"
	"sync"
	"testing"

	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var (
	fakeRecorder = record.NewFakeRecorder(10)
)

func setupScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := secretsstorev1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}

func newSecret(name, namespace string, labels map[string]string, annotations map[string]string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          labels,
			Annotations:     annotations,
			ResourceVersion: "73659",
		},
	}
}

func newSecretProviderClassPodStatus(name, namespace, node string) *secretsstorev1.SecretProviderClassPodStatus {
	return &secretsstorev1.SecretProviderClassPodStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          map[string]string{secretsstorev1.InternalNodeLabel: node},
			UID:             "72a0ecb8-c6e5-41e1-8da1-25e37ec61b26",
			ResourceVersion: "73659",
		},
		Status: secretsstorev1.SecretProviderClassPodStatusStatus{
			PodName:                 "pod1",
			TargetPath:              "/var/lib/kubelet/pods/d8771ddf-935a-4199-a20b-f35f71c1d9e7/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			SecretProviderClassName: "spc1",
			Mounted:                 true,
		},
	}
}

func newSecretProviderClass(name, namespace string) *secretsstorev1.SecretProviderClass {
	return &secretsstorev1.SecretProviderClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: secretsstorev1.SecretProviderClassSpec{
			Provider: "provider1",
			SecretObjects: []*secretsstorev1.SecretObject{
				{
					SecretName: "secret1",
					Type:       "Opaque",
				},
			},
		},
	}
}

func newPod(name, namespace string, owners []metav1.OwnerReference) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: owners,
		},
	}
}

func newReconciler(client client.Client, scheme *runtime.Scheme, nodeID string) *SecretProviderClassPodStatusReconciler {
	return &SecretProviderClassPodStatusReconciler{
		Client:        client,
		reader:        client,
		writer:        client,
		scheme:        scheme,
		eventRecorder: fakeRecorder,
		mutex:         &sync.Mutex{},
		nodeID:        nodeID,
		driverName:    "secrets-store.csi.k8s.io",
	}
}

// expectOwnerRefs asserts the secret's owner references match want exactly, by
// full-struct equality and independent of order. ConsistOf uses DeepEqual, so
// pointer fields (Controller, BlockOwnerDeletion) are compared by their pointees.
func expectOwnerRefs(g Gomega, secret *corev1.Secret, want ...metav1.OwnerReference) {
	g.Expect(secret.OwnerReferences).To(ConsistOf(want))
}

func TestPatchSecretWithOwnerRef(t *testing.T) {
	g := NewWithT(t)

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	spcPodStatus := newSecretProviderClassPodStatus("my-spcps", "default", "node1")
	// Create a new owner ref.
	gvk, err := apiutil.GVKForObject(spcPodStatus, scheme)
	g.Expect(err).NotTo(HaveOccurred())

	ref := metav1.OwnerReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		UID:        spcPodStatus.GetUID(),
		Name:       spcPodStatus.GetName(),
	}
	labels := map[string]string{"environment": "test"}
	annotations := map[string]string{"kubed.appscode.com/sync": "app=test"}

	initObjects := []client.Object{
		newSecret("my-secret", "default", labels, annotations),
		spcPodStatus,
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()
	reconciler := newReconciler(client, scheme, "node1")

	// adding ref twice to test de-duplication of owner references when being set in the secret
	err = reconciler.patchSecretWithOwnerRef(context.TODO(), "my-secret", "default", ref, ref)
	g.Expect(err).NotTo(HaveOccurred())

	secret := &corev1.Secret{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "my-secret", Namespace: "default"}, secret)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(secret.GetOwnerReferences()).To(HaveLen(1))
}

func TestCreateOrUpdateK8sSecret(t *testing.T) {
	g := NewWithT(t)

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	labels := map[string]string{"environment": "test"}
	annotations := map[string]string{"kubed.appscode.com/sync": "app=test"}

	initObjects := []client.Object{
		newSecret("my-secret", "default", labels, annotations),
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()
	reconciler := newReconciler(client, scheme, "node1")

	// secret already exists, just update it.
	err = reconciler.createOrUpdateK8sSecret(context.TODO(), "my-secret", "default", nil, labels, annotations, corev1.SecretTypeOpaque)
	g.Expect(err).NotTo(HaveOccurred())

	err = reconciler.createOrUpdateK8sSecret(context.TODO(), "my-secret2", "default", nil, labels, annotations, corev1.SecretTypeOpaque)
	g.Expect(err).NotTo(HaveOccurred())
	secret := &corev1.Secret{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "my-secret2", Namespace: "default"}, secret)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(secret.Labels).To(Equal(labels))

	g.Expect(secret.Name).To(Equal("my-secret2"))
}

func TestCreateOrUpdateHotloop(t *testing.T) {
	g := NewWithT(t)

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	labels := map[string]string{"environment": "test"}
	annotations := map[string]string{"kubed.appscode.com/sync": "app=test"}

	// secret1 is intentionally NOT pre-seeded so the create path runs.
	initObjects := []client.Object{
		newSecretProviderClassPodStatus("pod1-default-spc1", "default", "node1"),
		newSecretProviderClass("spc1", "default"),
		newPod("pod1", "default", []metav1.OwnerReference{
			{
				APIVersion:         "apps/v1",
				BlockOwnerDeletion: new(true),
				Controller:         new(true),
				Kind:               "ReplicaSet",
				Name:               "pod-6886c65f8f",
				UID:                "f39da13d-7246-4ef5-aed4-a6905f82cbcd",
			},
		}),
	}

	var createCount, updateCount, patchCount int
	client := fake.
		NewClientBuilder().
		WithScheme(scheme).
		WithObjects(initObjects...).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				createCount++
				return client.Create(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				updateCount++
				return client.Update(ctx, obj, opts...)
			},
			Patch: func(ctx context.Context, client client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				patchCount++
				return client.Patch(ctx, obj, patch, opts...)
			},
		}).
		Build()

	reconciler := newReconciler(client, scheme, "node1")

	// The Patcher copies only APIVersion/Kind/UID/Name from the pod's owner
	// references; Controller/BlockOwnerDeletion are intentionally left nil.
	wantRef := metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "ReplicaSet",
		Name:       "pod-6886c65f8f",
		UID:        "f39da13d-7246-4ef5-aed4-a6905f82cbcd",
	}

	secretKey := types.NamespacedName{Name: "secret1", Namespace: "default"}
	getSecret := func() *corev1.Secret {
		s := &corev1.Secret{}
		g.Expect(client.Get(context.TODO(), secretKey, s)).NotTo(HaveOccurred())
		return s
	}

	err = reconciler.createOrUpdateK8sSecret(context.TODO(), "secret1", "default", map[string][]byte{"foo": []byte("bar")}, labels, annotations, corev1.SecretTypeOpaque)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(createCount).To(Equal(1))
	g.Expect(updateCount).To(Equal(0))
	g.Expect(patchCount).To(Equal(0))

	err = reconciler.Patcher(context.TODO())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(createCount).To(Equal(1))
	g.Expect(updateCount).To(Equal(0))
	g.Expect(patchCount).To(Equal(1))

	s1 := getSecret()
	expectOwnerRefs(g, s1, wantRef)

	updateDataMap := map[string][]byte{"foo": []byte("baz")}
	// update the secret with new content
	err = reconciler.createOrUpdateK8sSecret(context.TODO(), "secret1", "default", updateDataMap, labels, annotations, corev1.SecretTypeOpaque)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(createCount).To(Equal(1))
	g.Expect(updateCount).To(Equal(0))
	g.Expect(patchCount).To(Equal(2))

	// owner refs should be preserved
	s2 := getSecret()
	expectOwnerRefs(g, s2, wantRef)

	// patching should be a no-op
	err = reconciler.Patcher(context.TODO())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(createCount).To(Equal(1))
	g.Expect(updateCount).To(Equal(0))
	g.Expect(patchCount).To(Equal(2))

	// attempt to update with unchanged data results in no-op (early return,
	// i.e. no patch/update)
	err = reconciler.createOrUpdateK8sSecret(context.TODO(), "secret1", "default", updateDataMap, labels, annotations, corev1.SecretTypeOpaque)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(createCount).To(Equal(1))
	g.Expect(updateCount).To(Equal(0))
	g.Expect(patchCount).To(Equal(2))

	s3 := getSecret()
	expectOwnerRefs(g, s3, wantRef)
	g.Expect(s3.Name).To(Equal("secret1"))
	g.Expect(s3.Labels).To(Equal(labels))
	g.Expect(s3.Annotations).To(Equal(annotations))
	g.Expect(s3.Data).To(Equal(map[string][]byte{"foo": []byte("baz")}))
}

func TestDataPatch(t *testing.T) {
	g := NewWithT(t)

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	labels := map[string]string{"environment": "test"}
	annotations := map[string]string{"kubed.appscode.com/sync": "app=test"}

	secretKey := types.NamespacedName{Name: "secret1", Namespace: "default"}
	initSecret := newSecret(secretKey.Name, secretKey.Namespace, labels, annotations)
	initSecret.Data = map[string][]byte{
		"key1": []byte("key1origvalue"),
		"key2": []byte("key2notdeleted"),
	}

	updateDataMap := map[string][]byte{
		"key1": []byte("key1replaced"),
		"key3": []byte("key3newkey"),
	}

	initObjects := []client.Object{
		initSecret,
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()
	reconciler := newReconciler(client, scheme, "node1")

	err = reconciler.createOrUpdateK8sSecret(context.TODO(), secretKey.Name, secretKey.Namespace, updateDataMap, labels, annotations, corev1.SecretTypeOpaque)
	g.Expect(err).NotTo(HaveOccurred())

	updated := &corev1.Secret{}
	err = client.Get(context.TODO(), secretKey, updated)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(updated.Data).To(Equal(updateDataMap))
}

// TestUpdateOverwritesForeignLabelsAndAnnotations proves that when another controller modifies
// only labels/annotations and adds its own owner ref, our controller notices
// (reconcile + patch run) and overwrites the foreign labels and annotations.
// TODO: we intend to change this in a future release as this behavior
// is incorrect.
func TestUpdateDoesNotModifyLabelsOrAnnotations(t *testing.T) {
	g := NewWithT(t)

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	// driver* is what our SecretObject declares; the foreign* keys are added by a
	// different controller and must not be stomped.
	foreignLabels := map[string]string{
		"key1": "key1foreignlabel", // value will be merged
		"key2": "key2foreignlabel",
	}
	driverLabels := map[string]string{
		"key1": "key1driverlabel",
		"key3": "key3driverlabel",
	}
	foreignAnnotations := map[string]string{
		"key1": "key1foreignannotation",
		"key2": "key2foreignannotation",
	}
	driverAnnotations := map[string]string{
		"key1": "key1driverannotation",
		"key3": "key3driverannotation",
	}

	foreignRef := metav1.OwnerReference{
		APIVersion: "example.com/v1",
		Kind:       "ExternalThing",
		Name:       "external-owner",
		UID:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	}
	foreignSecret := newSecret("secret1", "default", foreignLabels, foreignAnnotations)
	foreignSecret.OwnerReferences = []metav1.OwnerReference{foreignRef}

	initObjects := []client.Object{
		newSecretProviderClassPodStatus("pod1-default-spc1", "default", "node1"),
		newSecretProviderClass("spc1", "default"),
		newPod("pod1", "default", []metav1.OwnerReference{
			{
				APIVersion:         "apps/v1",
				BlockOwnerDeletion: new(true),
				Controller:         new(true),
				Kind:               "ReplicaSet",
				Name:               "pod-6886c65f8f",
				UID:                "f39da13d-7246-4ef5-aed4-a6905f82cbcd",
			},
		}),
		foreignSecret,
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()
	reconciler := newReconciler(client, scheme, "node1")

	driverRef := metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "ReplicaSet",
		Name:       "pod-6886c65f8f",
		UID:        "f39da13d-7246-4ef5-aed4-a6905f82cbcd",
	}

	secretKey := types.NamespacedName{Name: "secret1", Namespace: "default"}
	getSecret := func() *corev1.Secret {
		s := &corev1.Secret{}
		g.Expect(client.Get(context.TODO(), secretKey, s)).NotTo(HaveOccurred())
		return s
	}

	// annotations should not be modified with real update
	err = reconciler.createOrUpdateK8sSecret(context.TODO(), "secret1", "default", map[string][]byte{"foo": []byte("bar")}, driverLabels, driverAnnotations, corev1.SecretTypeOpaque)
	g.Expect(err).NotTo(HaveOccurred())
	err = reconciler.Patcher(context.TODO())
	g.Expect(err).NotTo(HaveOccurred())

	s1 := getSecret()
	expectOwnerRefs(g, s1, foreignRef, driverRef)
	g.Expect(s1.Labels).To(Equal(foreignLabels))
	g.Expect(s1.Annotations).To(Equal(foreignAnnotations))

	// no-op update should leave labels and annotations alone
	err = reconciler.createOrUpdateK8sSecret(context.TODO(), "secret1", "default", map[string][]byte{"foo": []byte("bar")}, driverLabels, driverAnnotations, corev1.SecretTypeOpaque)
	s2 := getSecret()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(s2.Labels).To(Equal(foreignLabels))
	g.Expect(s2.Annotations).To(Equal(foreignAnnotations))

	// patcher should leave labels and annotations alone
	err = reconciler.Patcher(context.TODO())
	g.Expect(err).NotTo(HaveOccurred())

	s3 := getSecret()
	expectOwnerRefs(g, s3, foreignRef, driverRef)
	g.Expect(s3.Labels).To(Equal(foreignLabels))
	g.Expect(s3.Annotations).To(Equal(foreignAnnotations))
}

func TestGenerateEvent(t *testing.T) {
	g := NewWithT(t)

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := newReconciler(client, scheme, "node1")

	obj := &corev1.ObjectReference{
		Name:      "pod1",
		Namespace: "default",
		UID:       "481ab824-1f07-4611-bc08-c41f5cbb5a8d",
	}

	reconciler.generateEvent(obj, corev1.EventTypeWarning, "reason", "message")
	reconciler.generateEvent(obj, corev1.EventTypeWarning, "reason2", "message2")

	event := <-fakeRecorder.Events
	g.Expect(event).To(Equal("Warning reason message"))
	event = <-fakeRecorder.Events
	g.Expect(event).To(Equal("Warning reason2 message2"))
}

func TestPatcherForStaticPod(t *testing.T) {
	g := NewWithT(t)

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	initObjects := []client.Object{
		newSecretProviderClassPodStatus("pod1-default-spc1", "default", "node1"),
		newSecretProviderClass("spc1", "default"),
		newPod("pod1", "default", nil),
		newSecret("secret1", "default", nil, nil),
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()
	reconciler := newReconciler(client, scheme, "node1")

	err = reconciler.Patcher(context.TODO())
	g.Expect(err).NotTo(HaveOccurred())

	// check the spcps has been added as owner to the secret
	secret := &corev1.Secret{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "secret1", Namespace: "default"}, secret)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(len(secret.OwnerReferences)).To(Equal(1))
	g.Expect(secret.OwnerReferences[0].APIVersion).To(Equal(secretsstorev1.GroupVersion.String()))
	g.Expect(secret.OwnerReferences[0].Kind).To(Equal("SecretProviderClassPodStatus"))
	g.Expect(secret.OwnerReferences[0].Name).To(Equal("pod1-default-spc1"))
}

func TestPatcherForPodWithOwner(t *testing.T) {
	g := NewWithT(t)

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	initObjects := []client.Object{
		newSecretProviderClassPodStatus("pod1-default-spc1", "default", "node1"),
		newSecretProviderClass("spc1", "default"),
		newPod("pod1", "default", []metav1.OwnerReference{
			{
				APIVersion:         "apps/v1",
				BlockOwnerDeletion: new(true),
				Controller:         new(true),
				Kind:               "ReplicaSet",
				Name:               "pod-6886c65f8f",
				UID:                "f39da13d-7246-4ef5-aed4-a6905f82cbcd",
			},
		}),
		newSecret("secret1", "default", nil, nil),
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()
	reconciler := newReconciler(client, scheme, "node1")

	err = reconciler.Patcher(context.TODO())
	g.Expect(err).NotTo(HaveOccurred())

	// check the spcps has been added as owner to the secret
	secret := &corev1.Secret{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "secret1", Namespace: "default"}, secret)
	g.Expect(err).NotTo(HaveOccurred())

	// Controller/BlockOwnerDeletion are intentionally dropped: the Patcher copies
	// only APIVersion/Kind/UID/Name from the pod's owner references.
	expectOwnerRefs(g, secret, metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "ReplicaSet",
		Name:       "pod-6886c65f8f",
		UID:        "f39da13d-7246-4ef5-aed4-a6905f82cbcd",
	})
}
