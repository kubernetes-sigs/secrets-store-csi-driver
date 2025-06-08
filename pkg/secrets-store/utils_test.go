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

package secretsstore

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/constants"
)

var (
	testPodName    = "pod-0"
	testNamespace  = "default"
	testPodUID     = "d8771ddf-935a-4199-a20b-f35f71c1d9e7"
	testSPCName    = "spc-0"
	testTargetPath = "/var/lib/kubelet/d8771ddf-935a-4199-a20b-f35f71c1d9e7/volumes/kubernetes.io~csi/secrets-store-inline/mount"
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

func TestCreateOrUpdateSecretProviderClassPodStatus(t *testing.T) {
	tests := []struct {
		name   string
		nodeID string
		// initial objects to add to the fake client
		initObjects []client.Object
		objects     map[string]string
	}{
		{
			name:        "create",
			nodeID:      "test-node",
			initObjects: []client.Object{},
			objects: map[string]string{
				"b": "v1",
				"a": "v2",
			},
		},
		{
			name:   "update",
			nodeID: "test-node",
			initObjects: []client.Object{
				newSecretProviderClassPodStatus(fmt.Sprintf("%s-%s-%s", testPodName, testNamespace, testSPCName), testNamespace, "old-node"),
			},
			objects: map[string]string{
				"b": "v1",
				"a": "v2",
			},
		},
	}

	want := &secretsstorev1.SecretProviderClassPodStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", testPodName, testNamespace, testSPCName),
			Namespace: testNamespace,
			Labels:    map[string]string{secretsstorev1.InternalNodeLabel: "test-node"},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       testPodName,
					UID:        types.UID(testPodUID),
				},
			},
		},
		Status: secretsstorev1.SecretProviderClassPodStatusStatus{
			PodName:                 testPodName,
			TargetPath:              testTargetPath,
			SecretProviderClassName: testSPCName,
			Mounted:                 true,
			Objects: []secretsstorev1.SecretProviderClassObject{
				{
					ID:      "a",
					Version: "v2",
				},
				{
					ID:      "b",
					Version: "v1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme, _ := setupScheme()
			cb := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.initObjects...)
			client := cb.Build()

			err := createOrUpdateSecretProviderClassPodStatus(context.TODO(), client, client, testPodName, testNamespace, testPodUID, testSPCName, testTargetPath, tt.nodeID, true, tt.objects, constants.NoGID)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			got := &secretsstorev1.SecretProviderClassPodStatus{}
			if err := client.Get(context.TODO(), types.NamespacedName{
				Name:      want.Name,
				Namespace: want.Namespace,
			}, got); err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got.GetLabels(), want.GetLabels()) {
				t.Errorf("ObjectMeta.GetLabels() got: %v, want: %v", got.GetLabels(), want.GetLabels())
			}
			if !reflect.DeepEqual(got.GetOwnerReferences(), want.GetOwnerReferences()) {
				t.Errorf("ObjectMeta.GetOwnerReferences() got: %v, want: %v", got.GetOwnerReferences(), want.GetOwnerReferences())
			}
			if !reflect.DeepEqual(got.Status, want.Status) {
				t.Errorf("Status got: %v, want: %v", got.Status, want.Status)
			}
		})
	}
}
