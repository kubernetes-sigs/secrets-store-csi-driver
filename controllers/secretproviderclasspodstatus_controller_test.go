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
	"testing"

	. "github.com/onsi/gomega"

	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
)

func setupScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}

func newSecret(name, namespace string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func newSecretProviderClassPodStatus(name, namespace, node string) *v1alpha1.SecretProviderClassPodStatus {
	return &v1alpha1.SecretProviderClassPodStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{internalNodeLabel: node},
			UID:       "72a0ecb8-c6e5-41e1-8da1-25e37ec61b26",
		},
		Status: v1alpha1.SecretProviderClassPodStatusStatus{
			PodName:                 "pod1",
			TargetPath:              "/var/lib/kubelet/pods/d8771ddf-935a-4199-a20b-f35f71c1d9e7/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			SecretProviderClassName: "spc1",
			Mounted:                 true,
		},
	}
}

func newReconciler(client client.Client, scheme *runtime.Scheme) *SecretProviderClassPodStatusReconciler {
	return &SecretProviderClassPodStatusReconciler{
		Client: client,
		Reader: client,
		Writer: client,
		Log:    log.New(),
		Scheme: scheme,
	}
}

func TestSecretExists(t *testing.T) {
	g := NewWithT(t)

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	initObjects := []runtime.Object{
		newSecret("my-secret", "default"),
	}

	client := fake.NewFakeClientWithScheme(scheme, initObjects...)
	reconciler := newReconciler(client, scheme)

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

	initObjects := []runtime.Object{
		newSecret("my-secret", "default"),
		spcPodStatus,
	}
	client := fake.NewFakeClientWithScheme(scheme, initObjects...)
	reconciler := newReconciler(client, scheme)

	err = reconciler.patchSecretWithOwnerRef(context.TODO(), "my-secret", "default", spcPodStatus)
	g.Expect(err).NotTo(HaveOccurred())

	secret := &v1.Secret{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "my-secret", Namespace: "default"}, secret)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(secret.GetOwnerReferences()).To(HaveLen(1))
}

func TestCreateK8sSecret(t *testing.T) {
	g := NewWithT(t)

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	initObjects := []runtime.Object{
		newSecret("my-secret", "default"),
	}
	client := fake.NewFakeClientWithScheme(scheme, initObjects...)
	reconciler := newReconciler(client, scheme)

	// secret already exists
	err = reconciler.createK8sSecret(context.TODO(), "my-secret", "default", nil, v1.SecretTypeOpaque)
	g.Expect(err).NotTo(HaveOccurred())

	err = reconciler.createK8sSecret(context.TODO(), "my-secret2", "default", nil, v1.SecretTypeOpaque)
	g.Expect(err).NotTo(HaveOccurred())
	secret := &v1.Secret{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: "my-secret2", Namespace: "default"}, secret)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(secret.Name).To(Equal("my-secret2"))
}
