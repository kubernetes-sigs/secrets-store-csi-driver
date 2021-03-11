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
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"

	"k8s.io/apimachinery/pkg/util/wait"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/fake"

	. "github.com/onsi/gomega"

	secretsStoreFakeClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
)

func TestGetPod(t *testing.T) {
	g := NewWithT(t)

	kubeClient := fake.NewSimpleClientset()
	crdClient := secretsStoreFakeClient.NewSimpleClientset()

	testStore, err := New(kubeClient, crdClient, "node1", 1*time.Millisecond, false)
	g.Expect(err).NotTo(HaveOccurred())
	err = testStore.Run(wait.NeverStop)
	g.Expect(err).NotTo(HaveOccurred())

	// Get a pod that's not found
	_, err = testStore.GetPod("pod1", "default")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	// add pod and then perform a get to ensure the correct pod is returned
	podToCreate := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			NodeName: "node1",
		},
	}

	_, err = kubeClient.CoreV1().Pods("default").Create(context.TODO(), podToCreate, metav1.CreateOptions{})
	g.Expect(err).NotTo(HaveOccurred())

	waitForInformerCacheSync()
	// Get pod1 now and it should be found in the cache
	pod, err := testStore.GetPod("pod1", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(pod).NotTo(BeNil())
	g.Expect(pod.Name).To(Equal("pod1"))
}

func TestListSecretProviderClassPodStatus(t *testing.T) {
	g := NewWithT(t)

	kubeClient := fake.NewSimpleClientset()
	crdClient := secretsStoreFakeClient.NewSimpleClientset()

	testStore, err := New(kubeClient, crdClient, "node1", 1*time.Millisecond, false)
	g.Expect(err).NotTo(HaveOccurred())
	err = testStore.Run(wait.NeverStop)
	g.Expect(err).NotTo(HaveOccurred())

	// Get a secretproviderclasspodstatus that's not found
	_, err = testStore.GetSecretProviderClassPodStatus("spcps1")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	secretProviderClassPodStatusToAdd := []*v1alpha1.SecretProviderClassPodStatus{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "spcpodstatus1",
				Namespace: "default",
				Labels:    map[string]string{v1alpha1.InternalNodeLabel: "node1"},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "spcpodstatus2",
				Namespace: "default",
				Labels:    map[string]string{v1alpha1.InternalNodeLabel: "node1"},
			},
		},
	}

	for _, spcps := range secretProviderClassPodStatusToAdd {
		_, err = crdClient.SecretsstoreV1alpha1().SecretProviderClassPodStatuses("default").Create(context.TODO(), spcps, metav1.CreateOptions{})
		g.Expect(err).NotTo(HaveOccurred())
	}

	waitForInformerCacheSync()

	list, err := testStore.ListSecretProviderClassPodStatus()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(len(list)).To(Equal(2))
	g.Expect(list).To(ConsistOf(secretProviderClassPodStatusToAdd))
}

func TestGetSecret(t *testing.T) {
	g := NewWithT(t)

	kubeClient := fake.NewSimpleClientset()
	crdClient := secretsStoreFakeClient.NewSimpleClientset()

	testStore, err := New(kubeClient, crdClient, "node1", 1*time.Millisecond, false)
	g.Expect(err).NotTo(HaveOccurred())
	err = testStore.Run(wait.NeverStop)
	g.Expect(err).NotTo(HaveOccurred())

	// Get a secret that's not found
	_, err = testStore.GetSecret("secret1", "default")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	secretToAdd := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret1",
			Namespace: "default",
		},
	}

	_, err = kubeClient.CoreV1().Secrets("default").Create(context.TODO(), secretToAdd, metav1.CreateOptions{})
	g.Expect(err).NotTo(HaveOccurred())

	waitForInformerCacheSync()

	secret, err := testStore.GetSecret("secret1", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(secret).NotTo(BeNil())
	g.Expect(secret.Name).To(Equal("secret1"))
}

func TestGetSecretProviderClass(t *testing.T) {
	g := NewWithT(t)

	kubeClient := fake.NewSimpleClientset()
	crdClient := secretsStoreFakeClient.NewSimpleClientset()

	testStore, err := New(kubeClient, crdClient, "node1", 1*time.Millisecond, false)
	g.Expect(err).NotTo(HaveOccurred())
	err = testStore.Run(wait.NeverStop)
	g.Expect(err).NotTo(HaveOccurred())

	// Get a spc that's not found
	_, err = testStore.GetSecretProviderClass("spc1", "default")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	secretProviderClassToAdd := &v1alpha1.SecretProviderClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spc1",
			Namespace: "default",
		},
	}

	_, err = crdClient.SecretsstoreV1alpha1().SecretProviderClasses("default").Create(context.TODO(), secretProviderClassToAdd, metav1.CreateOptions{})
	g.Expect(err).NotTo(HaveOccurred())

	waitForInformerCacheSync()

	spc, err := testStore.GetSecretProviderClass("spc1", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(spc).NotTo(BeNil())
	g.Expect(spc.Name).To(Equal("spc1"))
}

// waitForInformerCacheSync waits for the test informers cache to be synced
func waitForInformerCacheSync() {
	time.Sleep(200 * time.Millisecond)
}
