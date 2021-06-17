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
	"sigs.k8s.io/secrets-store-csi-driver/controllers"

	"k8s.io/apimachinery/pkg/util/wait"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes/fake"

	. "github.com/onsi/gomega"
)

func TestGetSecret(t *testing.T) {
	g := NewWithT(t)

	kubeClient := fake.NewSimpleClientset()

	testStore, err := New(kubeClient, 1*time.Millisecond, false)
	g.Expect(err).NotTo(HaveOccurred())
	err = testStore.Run(wait.NeverStop)
	g.Expect(err).NotTo(HaveOccurred())

	// Get a secret that's not found
	_, err = testStore.GetNodePublishSecretRefSecret("secret1", "default")
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	secretToAdd := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret1",
			Namespace: "default",
			Labels: map[string]string{
				controllers.SecretUsedLabel: "true",
			},
		},
	}

	_, err = kubeClient.CoreV1().Secrets("default").Create(context.TODO(), secretToAdd, metav1.CreateOptions{})
	g.Expect(err).NotTo(HaveOccurred())

	waitForInformerCacheSync()

	secret, err := testStore.GetNodePublishSecretRefSecret("secret1", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(secret).NotTo(BeNil())
	g.Expect(secret.Name).To(Equal("secret1"))
}

// waitForInformerCacheSync waits for the test informers cache to be synced
func waitForInformerCacheSync() {
	time.Sleep(200 * time.Millisecond)
}
