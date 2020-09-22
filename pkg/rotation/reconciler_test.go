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
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"k8s.io/client-go/util/workqueue"

	"k8s.io/apimachinery/pkg/types"

	secretsstore "sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store"

	v1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"k8s.io/client-go/kubernetes/fake"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	secretsStoreFakeClient "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/fake"
	providerfake "sigs.k8s.io/secrets-store-csi-driver/provider/fake"

	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlRuntimeFake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/k8s"
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

func newTestReconciler(s *runtime.Scheme, kubeClient kubernetes.Interface, crdClient *secretsStoreFakeClient.Clientset, ctrlClient client.Client, rotationPollInterval time.Duration, socketPath string) (*Reconciler, error) {
	store, err := k8s.New(kubeClient, crdClient, "nodeName", 5*time.Second)
	if err != nil {
		return nil, err
	}

	return &Reconciler{
		store:                store,
		ctrlReaderClient:     ctrlClient,
		ctrlWriterClient:     ctrlClient,
		scheme:               s,
		providerVolumePath:   socketPath,
		rotationPollInterval: rotationPollInterval,
		providerClients:      map[string]*secretsstore.CSIProviderClient{},
		queue:                workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		reporter:             newStatsReporter(),
	}, nil
}

func TestReconcileError(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name                                  string
		rotationPollInterval                  time.Duration
		secretProviderClassPodStatusToProcess *v1alpha1.SecretProviderClassPodStatus
		secretProviderClassToAdd              *v1alpha1.SecretProviderClass
		podToAdd                              *v1.Pod
		socketPath                            string
		secretToAdd                           *v1.Secret
		expectedObjectVersions                map[string]string
		expectedErr                           bool
	}{
		{
			name:                 "secret provider class not found",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: &v1alpha1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1-default-spc1",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.InternalNodeLabel: "nodeName"},
				},
				Status: v1alpha1.SecretProviderClassPodStatusStatus{
					SecretProviderClassName: "spc1",
					PodName:                 "pod1",
				},
			},
			secretProviderClassToAdd: &v1alpha1.SecretProviderClass{},
			podToAdd:                 &v1.Pod{},
			socketPath:               getTempTestDir(t),
			secretToAdd:              &v1.Secret{},
			expectedErr:              true,
		},
		{
			name:                 "failed to get pod",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: &v1alpha1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1-default-spc1",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.InternalNodeLabel: "nodeName"},
				},
				Status: v1alpha1.SecretProviderClassPodStatusStatus{
					SecretProviderClassName: "spc1",
					PodName:                 "pod1",
				},
			},
			secretProviderClassToAdd: &v1alpha1.SecretProviderClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "spc1",
					Namespace: "default",
				},
				Spec: v1alpha1.SecretProviderClassSpec{
					SecretObjects: []*v1alpha1.SecretObject{
						{
							Data: []*v1alpha1.SecretObjectData{
								{
									ObjectName: "object1",
									Key:        "foo",
								},
							},
						},
					},
				},
			},
			podToAdd:    &v1.Pod{},
			socketPath:  getTempTestDir(t),
			secretToAdd: &v1.Secret{},
			expectedErr: true,
		},
		{
			name:                 "failed to get NodePublishSecretRef secret",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: &v1alpha1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1-default-spc1",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.InternalNodeLabel: "nodeName"},
				},
				Status: v1alpha1.SecretProviderClassPodStatusStatus{
					SecretProviderClassName: "spc1",
					PodName:                 "pod1",
				},
			},
			secretProviderClassToAdd: &v1alpha1.SecretProviderClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "spc1",
					Namespace: "default",
				},
				Spec: v1alpha1.SecretProviderClassSpec{
					SecretObjects: []*v1alpha1.SecretObject{
						{
							Data: []*v1alpha1.SecretObjectData{
								{
									ObjectName: "object1",
									Key:        "foo",
								},
							},
						},
					},
					Provider: "provider1",
				},
			},
			podToAdd: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "csi-volume",
							VolumeSource: v1.VolumeSource{
								CSI: &v1.CSIVolumeSource{
									Driver:           "secrets-store.csi.k8s.io",
									VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
									NodePublishSecretRef: &v1.LocalObjectReference{
										Name: "secret1",
									},
								},
							},
						},
					},
				},
			},
			socketPath:  getTempTestDir(t),
			secretToAdd: &v1.Secret{},
			expectedErr: true,
		},
		{
			name:                 "failed to create provider client",
			rotationPollInterval: 60 * time.Second,
			secretProviderClassPodStatusToProcess: &v1alpha1.SecretProviderClassPodStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1-default-spc1",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.InternalNodeLabel: "nodeName"},
				},
				Status: v1alpha1.SecretProviderClassPodStatusStatus{
					SecretProviderClassName: "spc1",
					PodName:                 "pod1",
					TargetPath:              getTempTestDir(t),
				},
			},
			secretProviderClassToAdd: &v1alpha1.SecretProviderClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "spc1",
					Namespace: "default",
				},
				Spec: v1alpha1.SecretProviderClassSpec{
					SecretObjects: []*v1alpha1.SecretObject{
						{
							Data: []*v1alpha1.SecretObjectData{
								{
									ObjectName: "object1",
									Key:        "foo",
								},
							},
						},
					},
					Provider: "wrongprovider",
				},
			},
			podToAdd: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					Volumes: []v1.Volume{
						{
							Name: "csi-volume",
							VolumeSource: v1.VolumeSource{
								CSI: &v1.CSIVolumeSource{
									Driver:           "secrets-store.csi.k8s.io",
									VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
									NodePublishSecretRef: &v1.LocalObjectReference{
										Name: "secret1",
									},
								},
							},
						},
					},
				},
			},
			socketPath: getTempTestDir(t),
			secretToAdd: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret1",
					Namespace: "default",
				},
				Data: map[string][]byte{"clientid": []byte("clientid")},
			},
			expectedErr: true,
		},
	}

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset(test.podToAdd, test.secretToAdd)
			crdClient := secretsStoreFakeClient.NewSimpleClientset(test.secretProviderClassPodStatusToProcess, test.secretProviderClassToAdd)
			ctrlClient := ctrlRuntimeFake.NewFakeClientWithScheme(scheme)

			testReconciler, err := newTestReconciler(scheme, kubeClient, crdClient, ctrlClient, test.rotationPollInterval, test.socketPath)
			g.Expect(err).NotTo(HaveOccurred())
			err = testReconciler.store.Run(wait.NeverStop)
			g.Expect(err).NotTo(HaveOccurred())

			serverEndpoint := fmt.Sprintf("%s/%s.sock", test.socketPath, "provider1")
			defer os.Remove(serverEndpoint)

			server, err := providerfake.NewMocKCSIProviderServer(serverEndpoint)
			g.Expect(err).NotTo(HaveOccurred())
			server.SetObjects(test.expectedObjectVersions)
			server.Start()

			err = testReconciler.reconcile(context.TODO(), test.secretProviderClassPodStatusToProcess)
			g.Expect(err).To(HaveOccurred())
		})
	}
}

func TestReconcileNoError(t *testing.T) {
	g := NewWithT(t)

	secretProviderClassPodStatusToProcess := &v1alpha1.SecretProviderClassPodStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1-default-spc1",
			Namespace: "default",
			Labels:    map[string]string{v1alpha1.InternalNodeLabel: "nodeName"},
		},
		Status: v1alpha1.SecretProviderClassPodStatusStatus{
			SecretProviderClassName: "spc1",
			PodName:                 "pod1",
			TargetPath:              getTempTestDir(t),
			Objects: []v1alpha1.SecretProviderClassObject{
				{
					ID:      "secret/object1",
					Version: "v1",
				},
			},
		},
	}
	secretProviderClassToAdd := &v1alpha1.SecretProviderClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spc1",
			Namespace: "default",
		},
		Spec: v1alpha1.SecretProviderClassSpec{
			SecretObjects: []*v1alpha1.SecretObject{
				{
					Data: []*v1alpha1.SecretObjectData{
						{
							ObjectName: "object1",
							Key:        "foo",
						},
					},
					SecretName: "foosecret",
					Type:       "Opaque",
				},
			},
			Provider: "provider1",
		},
	}
	podToAdd := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{
					Name: "csi-volume",
					VolumeSource: v1.VolumeSource{
						CSI: &v1.CSIVolumeSource{
							Driver:           "secrets-store.csi.k8s.io",
							VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
							NodePublishSecretRef: &v1.LocalObjectReference{
								Name: "secret1",
							},
						},
					},
				},
			},
		},
	}
	secretToAdd := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret1",
			Namespace: "default",
		},
		Data: map[string][]byte{"clientid": []byte("clientid")},
	}
	secretToBeRotated := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foosecret",
			Namespace:       "default",
			ResourceVersion: "rv1",
		},
		Data: map[string][]byte{"foo": []byte("olddata")},
	}
	socketPath := getTempTestDir(t)
	expectedObjectVersions := map[string]string{"secret/object1": "v2"}
	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	kubeClient := fake.NewSimpleClientset(podToAdd, secretToAdd)
	crdClient := secretsStoreFakeClient.NewSimpleClientset(secretProviderClassPodStatusToProcess, secretProviderClassToAdd)
	ctrlClient := ctrlRuntimeFake.NewFakeClientWithScheme(scheme, secretProviderClassPodStatusToProcess, secretToBeRotated)

	testReconciler, err := newTestReconciler(scheme, kubeClient, crdClient, ctrlClient, 60*time.Second, socketPath)
	g.Expect(err).NotTo(HaveOccurred())
	err = testReconciler.store.Run(wait.NeverStop)
	g.Expect(err).NotTo(HaveOccurred())

	serverEndpoint := fmt.Sprintf("%s/%s.sock", socketPath, "provider1")
	defer os.Remove(serverEndpoint)

	server, err := providerfake.NewMocKCSIProviderServer(serverEndpoint)
	g.Expect(err).NotTo(HaveOccurred())
	server.SetObjects(expectedObjectVersions)
	server.Start()

	err = ioutil.WriteFile(secretProviderClassPodStatusToProcess.Status.TargetPath+"/object1", []byte("newdata"), permission)
	g.Expect(err).NotTo(HaveOccurred())

	err = testReconciler.reconcile(context.TODO(), secretProviderClassPodStatusToProcess)
	g.Expect(err).NotTo(HaveOccurred())

	// validate the secret provider class pod status versions have been updated
	updatedSPCPodStatus := &v1alpha1.SecretProviderClassPodStatus{}
	err = ctrlClient.Get(context.TODO(), types.NamespacedName{Name: "pod1-default-spc1", Namespace: "default"}, updatedSPCPodStatus)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(updatedSPCPodStatus.Status.Objects).To(Equal([]v1alpha1.SecretProviderClassObject{{ID: "secret/object1", Version: "v2"}}))

	// validate the secret data has been updated to the latest value
	updatedSecret := &v1.Secret{}
	err = ctrlClient.Get(context.TODO(), types.NamespacedName{Name: "foosecret", Namespace: "default"}, updatedSecret)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(updatedSecret.Data["foo"]).To(Equal([]byte("newdata")))
}

func getTempTestDir(t *testing.T) string {
	tmpDir, err := ioutil.TempDir("", "ut")
	if err != nil {
		t.Fatalf("expected err to be nil, got: %+v", err)
	}
	return tmpDir
}
