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

package rotation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	"sigs.k8s.io/secrets-store-csi-driver/controllers"
	secretsStoreFakeClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/k8s"
	secretsstore "sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store"
	providerfake "sigs.k8s.io/secrets-store-csi-driver/provider/fake"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	fakeRecorder = record.NewFakeRecorder(20)
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

func newTestReconciler(client client.Reader, kubeClient kubernetes.Interface, crdClient *secretsStoreFakeClient.Clientset, rotationPollInterval time.Duration, socketPath string) (*Reconciler, error) {
	secretStore, err := k8s.New(kubeClient, 5*time.Second)
	if err != nil {
		return nil, err
	}
	sr, err := newStatsReporter()
	if err != nil {
		return nil, err
	}

	return &Reconciler{
		rotationPollInterval: rotationPollInterval,
		providerClients:      secretsstore.NewPluginClientBuilder([]string{socketPath}),
		queue:                workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		reporter:             sr,
		eventRecorder:        fakeRecorder,
		kubeClient:           kubeClient,
		crdClient:            crdClient,
		cache:                client,
		secretStore:          secretStore,
		tokenClient:          k8s.NewTokenClient(kubeClient, "test-driver", 1*time.Second),
		driverName:           "secrets-store.csi.k8s.io",
	}, nil
}

func getSPC(customize func(*secretsstorev1.SecretProviderClass)) *secretsstorev1.SecretProviderClass {
	var spc = &secretsstorev1.SecretProviderClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spc1",
			Namespace: "default",
		},
		Spec: secretsstorev1.SecretProviderClassSpec{
			SecretObjects: []*secretsstorev1.SecretObject{
				{
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "object1",
							Key:        "foo",
						},
					},
				},
			},
			Provider: "provider1",
		},
	}
	customize(spc)
	return spc
}

func getSPCPS(t *testing.T, customize func(*secretsstorev1.SecretProviderClassPodStatus)) *secretsstorev1.SecretProviderClassPodStatus {
	var spcps = &secretsstorev1.SecretProviderClassPodStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1-default-spc1",
			Namespace: "default",
			Labels:    map[string]string{secretsstorev1.InternalNodeLabel: "nodeName"},
		},
		Status: secretsstorev1.SecretProviderClassPodStatusStatus{
			SecretProviderClassName: "spc1",
			PodName:                 "pod1",
			TargetPath:              getTestTargetPath(t, "foo", "csi-volume"),
			Objects: []secretsstorev1.SecretProviderClassObject{
				{
					ID:      "secret/object1",
					Version: "v1",
				},
			},
		},
	}
	customize(spcps)
	return spcps
}

func getPod(customize func(*corev1.Pod)) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "default",
			UID:       types.UID("foo"),
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "csi-volume",
					VolumeSource: corev1.VolumeSource{
						CSI: &corev1.CSIVolumeSource{
							Driver:           "secrets-store.csi.k8s.io",
							VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
							NodePublishSecretRef: &corev1.LocalObjectReference{
								Name: "secret1",
							},
						},
					},
				},
			},
		},
	}
	customize(pod)
	return pod
}

func GetNodePublishSecretRefSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret1",
			Namespace: "default",
			Labels: map[string]string{
				controllers.SecretUsedLabel: "true",
			},
		},
		Data: map[string][]byte{"clientid": []byte("clientid")},
	}
}
func TestReconcileError(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name                                  string
		rotationPollInterval                  time.Duration
		secretProviderClassPodStatusToProcess *secretsstorev1.SecretProviderClassPodStatus
		secretProviderClassToAdd              *secretsstorev1.SecretProviderClass
		podToAdd                              *corev1.Pod
		socketPath                            string
		secretToAdd                           *corev1.Secret
		expectedObjectVersions                map[string]string
		expectedErr                           bool
		expectedErrorEvents                   bool
	}{
		{
			name:                                  "secret provider class not found",
			rotationPollInterval:                  60 * time.Second,
			secretProviderClassPodStatusToProcess: getSPCPS(t, func(*secretsstorev1.SecretProviderClassPodStatus) {}),
			secretProviderClassToAdd:              &secretsstorev1.SecretProviderClass{},
			podToAdd:                              &corev1.Pod{},
			socketPath:                            t.TempDir(),
			secretToAdd:                           &corev1.Secret{},
			expectedErr:                           true,
		},
		{
			name:                                  "failed to get pod",
			rotationPollInterval:                  60 * time.Second,
			secretProviderClassPodStatusToProcess: getSPCPS(t, func(*secretsstorev1.SecretProviderClassPodStatus) {}),
			secretProviderClassToAdd: getSPC(func(s *secretsstorev1.SecretProviderClass) {
				s.Spec.Provider = ""
			}),
			podToAdd:    &corev1.Pod{},
			socketPath:  t.TempDir(),
			secretToAdd: &corev1.Secret{},
			expectedErr: true,
		},
		{
			name:                                  "failed to get NodePublishSecretRef secret",
			rotationPollInterval:                  60 * time.Second,
			secretProviderClassPodStatusToProcess: getSPCPS(t, func(*secretsstorev1.SecretProviderClassPodStatus) {}),
			secretProviderClassToAdd:              getSPC(func(*secretsstorev1.SecretProviderClass) {}),
			podToAdd:                              getPod(func(*corev1.Pod) {}),
			socketPath:                            t.TempDir(),
			secretToAdd:                           &corev1.Secret{},
			expectedErr:                           true,
			expectedErrorEvents:                   true,
		},
		{
			name:                 "failed to validate targetpath UID",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: getSPCPS(t, func(s *secretsstorev1.SecretProviderClassPodStatus) {
				s.Status.TargetPath = getTestTargetPath(t, "bad-uid", "csi-volume")
			}),
			secretProviderClassToAdd: getSPC(func(*secretsstorev1.SecretProviderClass) {}),
			podToAdd:                 getPod(func(*corev1.Pod) {}),
			socketPath:               t.TempDir(),
			secretToAdd: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "object1",
					Namespace:       "default",
					ResourceVersion: "rv1",
				},
				Data: map[string][]byte{"foo": []byte("olddata")},
			},
			expectedObjectVersions: map[string]string{"secret/object1": "v2"},
			expectedErr:            true,
			expectedErrorEvents:    false,
		},
		{
			name:                 "failed to validate targetpath volume name",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: getSPCPS(t, func(s *secretsstorev1.SecretProviderClassPodStatus) {
				s.Status.TargetPath = getTestTargetPath(t, "foo", "bad-volume-name")
			}),
			secretProviderClassToAdd: getSPC(func(*secretsstorev1.SecretProviderClass) {}),
			podToAdd:                 getPod(func(*corev1.Pod) {}),
			socketPath:               t.TempDir(),
			secretToAdd: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "object1",
					Namespace:       "default",
					ResourceVersion: "rv1",
				},
				Data: map[string][]byte{"foo": []byte("olddata")},
			},
			expectedObjectVersions: map[string]string{"secret/object1": "v2"},
			expectedErr:            true,
			expectedErrorEvents:    false,
		},
		{
			name:                                  "failed to lookup provider client",
			rotationPollInterval:                  60 * time.Second,
			secretProviderClassPodStatusToProcess: getSPCPS(t, func(s *secretsstorev1.SecretProviderClassPodStatus) {}),
			secretProviderClassToAdd: getSPC(func(s *secretsstorev1.SecretProviderClass) {
				s.Spec.Provider = "wrongprovider"
			}),
			podToAdd:            getPod(func(*corev1.Pod) {}),
			socketPath:          t.TempDir(),
			secretToAdd:         GetNodePublishSecretRefSecret(),
			expectedErr:         true,
			expectedErrorEvents: true,
		},
		{
			name:                 "failed to parse FSGroup",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: getSPCPS(t, func(s *secretsstorev1.SecretProviderClassPodStatus) {
				s.Status.FSGroup = "INVALID"
			}),
			secretProviderClassToAdd: getSPC(func(*secretsstorev1.SecretProviderClass) {}),
			podToAdd:                 getPod(func(*corev1.Pod) {}),
			socketPath:               t.TempDir(),
			secretToAdd:              GetNodePublishSecretRefSecret(),
			expectedErr:              true,
			expectedErrorEvents:      true,
		},
	}

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset(test.podToAdd, test.secretToAdd)
			crdClient := secretsStoreFakeClient.NewSimpleClientset(test.secretProviderClassPodStatusToProcess, test.secretProviderClassToAdd)

			initObjects := []client.Object{
				test.podToAdd,
				test.secretToAdd,
				test.secretProviderClassPodStatusToProcess,
				test.secretProviderClassToAdd,
			}
			client := controllerfake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()

			testReconciler, err := newTestReconciler(client, kubeClient, crdClient, test.rotationPollInterval, test.socketPath)
			g.Expect(err).NotTo(HaveOccurred())

			err = testReconciler.secretStore.Run(wait.NeverStop)
			g.Expect(err).NotTo(HaveOccurred())

			serverEndpoint := fmt.Sprintf("%s/%s.sock", test.socketPath, "provider1")
			defer os.Remove(serverEndpoint)

			server, err := providerfake.NewMocKCSIProviderServer(serverEndpoint)
			g.Expect(err).NotTo(HaveOccurred())
			server.SetObjects(test.expectedObjectVersions)
			err = server.Start()
			g.Expect(err).NotTo(HaveOccurred())

			err = testReconciler.reconcile(context.TODO(), test.secretProviderClassPodStatusToProcess)
			g.Expect(err).To(HaveOccurred())
			if test.expectedErrorEvents {
				g.Expect(len(fakeRecorder.Events)).ToNot(BeNumerically("==", 0))
				for len(fakeRecorder.Events) > 0 {
					fmt.Println(<-fakeRecorder.Events)
				}
			}
		})
	}
}

func TestReconcileNoError(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name                                  string
		nodePublishSecretRefSecretToAdd       *corev1.Secret
		secretProviderClassPodStatusToProcess *secretsstorev1.SecretProviderClassPodStatus
	}{
		{
			name:                                  "filtered watch for nodePublishSecretRef",
			nodePublishSecretRefSecretToAdd:       GetNodePublishSecretRefSecret(),
			secretProviderClassPodStatusToProcess: getSPCPS(t, func(*secretsstorev1.SecretProviderClassPodStatus) {}),
		},
		{
			name:                            "reconcile with FSGroup",
			nodePublishSecretRefSecretToAdd: GetNodePublishSecretRefSecret(),
			secretProviderClassPodStatusToProcess: getSPCPS(t, func(s *secretsstorev1.SecretProviderClassPodStatus) {
				s.Status.FSGroup = "1004"
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			secretProviderClassPodStatusToProcess := test.secretProviderClassPodStatusToProcess
			secretProviderClassToAdd := getSPC(func(s *secretsstorev1.SecretProviderClass) {
				s.Spec.SecretObjects[0].SecretName = "foosecret"
				s.Spec.SecretObjects[0].Type = "Opaque"
			})
			podToAdd := getPod(func(*corev1.Pod) {})
			secretToBeRotated := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "foosecret",
					Namespace:       "default",
					ResourceVersion: "12352",
					Labels: map[string]string{
						controllers.SecretManagedLabel: "true",
					},
				},
				Data: map[string][]byte{"foo": []byte("olddata")},
			}

			socketPath := t.TempDir()
			expectedObjectVersions := map[string]string{"secret/object1": "v2"}
			scheme, err := setupScheme()
			g.Expect(err).NotTo(HaveOccurred())

			kubeClient := fake.NewSimpleClientset(podToAdd, test.nodePublishSecretRefSecretToAdd, secretToBeRotated)
			crdClient := secretsStoreFakeClient.NewSimpleClientset(secretProviderClassPodStatusToProcess, secretProviderClassToAdd)

			initObjects := []client.Object{
				podToAdd,
				secretToBeRotated,
				test.nodePublishSecretRefSecretToAdd,
				secretProviderClassPodStatusToProcess,
				secretProviderClassToAdd,
			}
			ctrlClient := controllerfake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()

			testReconciler, err := newTestReconciler(ctrlClient, kubeClient, crdClient, 60*time.Second, socketPath)
			g.Expect(err).NotTo(HaveOccurred())
			err = testReconciler.secretStore.Run(wait.NeverStop)
			g.Expect(err).NotTo(HaveOccurred())

			serverEndpoint := fmt.Sprintf("%s/%s.sock", socketPath, "provider1")
			defer os.Remove(serverEndpoint)

			server, err := providerfake.NewMocKCSIProviderServer(serverEndpoint)
			g.Expect(err).NotTo(HaveOccurred())
			server.SetObjects(expectedObjectVersions)
			err = server.Start()
			g.Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(secretProviderClassPodStatusToProcess.Status.TargetPath+"/object1", []byte("newdata"), secretsstore.FilePermission)
			g.Expect(err).NotTo(HaveOccurred())

			err = testReconciler.reconcile(context.TODO(), secretProviderClassPodStatusToProcess)
			g.Expect(err).NotTo(HaveOccurred())

			// validate the secret provider class pod status versions have been updated
			updatedSPCPodStatus, err := crdClient.SecretsstoreV1().SecretProviderClassPodStatuses(corev1.NamespaceDefault).Get(context.TODO(), "pod1-default-spc1", metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(updatedSPCPodStatus.Status.Objects).To(Equal([]secretsstorev1.SecretProviderClassObject{{ID: "secret/object1", Version: "v2"}}))

			// validate the secret data has been updated to the latest value
			updatedSecret, err := kubeClient.CoreV1().Secrets(corev1.NamespaceDefault).Get(context.TODO(), "foosecret", metav1.GetOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(updatedSecret.Data["foo"]).To(Equal([]byte("newdata")))

			// 2 normal events - one for successfully updating the mounted contents and
			// second for successfully rotating the K8s secret
			g.Expect(len(fakeRecorder.Events)).To(BeNumerically("==", 2))
			for len(fakeRecorder.Events) > 0 {
				<-fakeRecorder.Events
			}

			// test with pod being terminated
			podToAdd.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			kubeClient = fake.NewSimpleClientset(podToAdd, test.nodePublishSecretRefSecretToAdd)
			initObjects = []client.Object{
				podToAdd,
				test.nodePublishSecretRefSecretToAdd,
			}
			ctrlClient = controllerfake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()
			testReconciler, err = newTestReconciler(ctrlClient, kubeClient, crdClient, 60*time.Second, socketPath)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(err).NotTo(HaveOccurred())

			err = testReconciler.reconcile(context.TODO(), secretProviderClassPodStatusToProcess)
			g.Expect(err).NotTo(HaveOccurred())

			// test with pod being in succeeded phase
			podToAdd.DeletionTimestamp = nil
			podToAdd.Status.Phase = corev1.PodSucceeded
			kubeClient = fake.NewSimpleClientset(podToAdd, test.nodePublishSecretRefSecretToAdd)
			initObjects = []client.Object{
				podToAdd,
				test.nodePublishSecretRefSecretToAdd,
			}
			ctrlClient = controllerfake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()
			testReconciler, err = newTestReconciler(ctrlClient, kubeClient, crdClient, 60*time.Second, socketPath)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(err).NotTo(HaveOccurred())

			err = testReconciler.reconcile(context.TODO(), secretProviderClassPodStatusToProcess)
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestPatchSecret(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name               string
		secretToAdd        *corev1.Secret
		secretName         string
		expectedSecretData map[string][]byte
		expectedErr        bool
	}{
		{
			name:        "secret is not found",
			secretToAdd: &corev1.Secret{},
			secretName:  "secret1",
			expectedErr: true,
		},
		{
			name: "secret is found and data already matches",
			secretToAdd: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "secret1",
					Namespace:       "default",
					ResourceVersion: "16172",
					Labels: map[string]string{
						controllers.SecretManagedLabel: "true",
					},
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			secretName:         "secret1",
			expectedSecretData: map[string][]byte{"key1": []byte("value1")},
			expectedErr:        false,
		},
		{
			name: "secret is found and data is updated to latest",
			secretToAdd: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "secret1",
					Namespace:       "default",
					ResourceVersion: "16172",
					Labels: map[string]string{
						controllers.SecretManagedLabel: "true",
					},
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			secretName:         "secret1",
			expectedSecretData: map[string][]byte{"key2": []byte("value2")},
			expectedErr:        false,
		},
		{
			name: "secret is found and new data is appended to existing",
			secretToAdd: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "secret1",
					Namespace:       "default",
					ResourceVersion: "16172",
					Labels: map[string]string{
						controllers.SecretManagedLabel: "true",
					},
				},
				Data: map[string][]byte{"key1": []byte("value1")},
			},
			secretName:         "secret1",
			expectedSecretData: map[string][]byte{"key1": []byte("value1"), "key2": []byte("value2")},
			expectedErr:        false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme, err := setupScheme()
			g.Expect(err).NotTo(HaveOccurred())

			kubeClient := fake.NewSimpleClientset(test.secretToAdd)
			crdClient := secretsStoreFakeClient.NewSimpleClientset()

			initObjects := []client.Object{
				test.secretToAdd,
			}
			ctrlClient := controllerfake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()

			testReconciler, err := newTestReconciler(ctrlClient, kubeClient, crdClient, 60*time.Second, "")
			g.Expect(err).NotTo(HaveOccurred())
			err = testReconciler.secretStore.Run(wait.NeverStop)
			g.Expect(err).NotTo(HaveOccurred())

			err = testReconciler.patchSecret(context.TODO(), test.secretName, corev1.NamespaceDefault, test.expectedSecretData)
			if test.expectedErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			if !test.expectedErr {
				// check the secret data is what we expect it to
				secret, err := kubeClient.CoreV1().Secrets(corev1.NamespaceDefault).Get(context.TODO(), test.secretName, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(secret.Data).To(Equal(test.expectedSecretData))
			}
		})
	}
}

func TestHandleError(t *testing.T) {
	g := NewWithT(t)

	testReconciler, err := newTestReconciler(nil, nil, nil, 60*time.Second, "")
	g.Expect(err).NotTo(HaveOccurred())

	testReconciler.handleError(errors.New("failed error"), "key1", false)
	// wait for the object to be requeued
	time.Sleep(11 * time.Second)
	g.Expect(testReconciler.queue.Len()).To(Equal(1))

	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		testReconciler.handleError(errors.New("failed error"), "key1", true)
		g.Expect(testReconciler.queue.NumRequeues("key1")).To(Equal(i + 1))

		testReconciler.queue.Get()
		testReconciler.queue.Done("key1")
	}

	// max number of requeues complete for key2, so now it should be removed from queue
	testReconciler.handleError(errors.New("failed error"), "key1", true)
	time.Sleep(1 * time.Second)
	g.Expect(testReconciler.queue.Len()).To(Equal(1))
}

func getTestTargetPath(t *testing.T, uid, vol string) string {
	path := filepath.Join(t.TempDir(), "pods", uid, "volumes", "kubernetes.io~csi", vol, "mount")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}
	return path
}
