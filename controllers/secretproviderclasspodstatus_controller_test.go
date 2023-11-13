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

func TestSecretExists(t *testing.T) {
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

	exists, err := reconciler.secretExists(context.TODO(), "my-secret", "default")
	g.Expect(exists).To(Equal(true))
	g.Expect(err).NotTo(HaveOccurred())

	exists, err = reconciler.secretExists(context.TODO(), "my-secret2", "default")
	g.Expect(exists).To(Equal(false))
	g.Expect(err).NotTo(HaveOccurred())
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

func TestCreateK8sSecret(t *testing.T) {
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

	// secret already exists
	err = reconciler.createK8sSecret(context.TODO(), "my-secret", "default", nil, labels, annotations, corev1.SecretTypeOpaque)
	g.Expect(err).NotTo(HaveOccurred())

	err = reconciler.createK8sSecret(context.TODO(), "my-secret2", "default", nil, labels, annotations, corev1.SecretTypeOpaque)
	g.Expect(err).NotTo(HaveOccurred())
	secret := &corev1.Secret{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "my-secret2", Namespace: "default"}, secret)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(secret.Labels).To(Equal(labels))

	g.Expect(secret.Name).To(Equal("my-secret2"))
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
	tr := true

	initObjects := []client.Object{
		newSecretProviderClassPodStatus("pod1-default-spc1", "default", "node1"),
		newSecretProviderClass("spc1", "default"),
		newPod("pod1", "default", []metav1.OwnerReference{
			{
				APIVersion:         "apps/v1",
				BlockOwnerDeletion: &tr,
				Controller:         &tr,
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

	g.Expect(len(secret.OwnerReferences)).To(Equal(1))
	g.Expect(secret.OwnerReferences[0].APIVersion).To(Equal("apps/v1"))
	g.Expect(secret.OwnerReferences[0].Kind).To(Equal("ReplicaSet"))
	g.Expect(secret.OwnerReferences[0].Name).To(Equal("pod-6886c65f8f"))
	g.Expect(secret.OwnerReferences[0].UID).To(Equal(types.UID("f39da13d-7246-4ef5-aed4-a6905f82cbcd")))
}
