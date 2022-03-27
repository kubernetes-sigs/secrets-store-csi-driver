/*
Copyright 2021 The Kubernetes Authors.

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

package spcutil

import (
	"reflect"
	"sort"
	"testing"

	corev1 "k8s.io/api/core/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

func TestBuildSecretObjects(t *testing.T) {
	tests := []struct {
		files      map[string]string
		secretType corev1.SecretType
		expected   []*secretsstorev1.SecretObject
	}{
		{
			files: map[string]string{
				"username":               "/some/path/on/node/username",
				"password":               "/some/path/on/node/password",
				"nested/username":        "/some/path/on/node/nested/username",
				"nested/double/username": "/some/path/on/node/double/nested/username",
			},
			secretType: corev1.SecretTypeOpaque,
			expected: []*secretsstorev1.SecretObject{
				{
					SecretName: "username",
					Type:       string(corev1.SecretTypeOpaque),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "username",
							Key:        "username",
						},
					},
				},
				{
					SecretName: "password",
					Type:       string(corev1.SecretTypeOpaque),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "password",
							Key:        "password",
						},
					},
				},
				{
					SecretName: "nested-username",
					Type:       string(corev1.SecretTypeOpaque),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "nested/username",
							Key:        "username",
						},
					},
				},
				{
					SecretName: "nested-double-username",
					Type:       string(corev1.SecretTypeOpaque),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "nested/double/username",
							Key:        "username",
						},
					},
				},
			},
		}, {
			files: map[string]string{
				"cert": "/some/path/on/node/cert",
			},
			secretType: corev1.SecretTypeTLS,
			expected: []*secretsstorev1.SecretObject{
				{
					SecretName: "cert",
					Type:       string(corev1.SecretTypeTLS),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "cert",
							Key:        "tls.key",
						},
						{
							ObjectName: "cert",
							Key:        "tls.crt",
						},
					},
				},
			},
		}, {
			files: map[string]string{
				"basic/basic1": "/some/path/on/node/basic/basic1",
				"basic/basic2": "/some/path/on/node/basic/basic1",
				"basic/basic3": "/some/path/on/node/basic/basic1",
			},
			secretType: corev1.SecretTypeBasicAuth,
			expected: []*secretsstorev1.SecretObject{
				{
					SecretName: "basic-basic1",
					Type:       string(corev1.SecretTypeBasicAuth),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "basic/basic1",
							Key:        "basic1",
						},
					},
				},
				{
					SecretName: "basic-basic2",
					Type:       string(corev1.SecretTypeBasicAuth),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "basic/basic2",
							Key:        "basic2",
						},
					},
				},
				{
					SecretName: "basic-basic3",
					Type:       string(corev1.SecretTypeBasicAuth),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "basic/basic3",
							Key:        "basic3",
						},
					},
				},
			},
		}, {
			files: map[string]string{
				"ssh": "/some/path/on/node/ssh",
			},
			secretType: corev1.SecretTypeSSHAuth,
			expected: []*secretsstorev1.SecretObject{
				{
					SecretName: "ssh",
					Type:       string(corev1.SecretTypeSSHAuth),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "ssh",
							Key:        "ssh-privatekey",
						},
					},
				},
			},
		},
		{
			files: map[string]string{
				"dev/docker": "/some/path/on/node/dev/docker",
			},
			secretType: corev1.SecretTypeDockerConfigJson,
			expected: []*secretsstorev1.SecretObject{
				{
					SecretName: "dev-docker",
					Type:       string(corev1.SecretTypeDockerConfigJson),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "dev/docker",
							Key:        ".dockerconfigjson",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		// TODO (manedurphy) Check back here
		actualSecretObjects := BuildSecretObjects(test.files, test.secretType)
		if ok := assertSecretObjectSlicesEqual(test.expected, actualSecretObjects); !ok {
			t.Fatal("secret object slices did not match")
		}
	}

}

func assertSecretsObjectsEqual(expected, actual *secretsstorev1.SecretObject) bool {
	if expected.SecretName != actual.SecretName {
		return false
	}

	if expected.Type != actual.Type {
		return false
	}

	sort.Slice(expected.Data, func(i, j int) bool {
		return expected.Data[i].ObjectName < expected.Data[j].ObjectName
	})

	sort.Slice(actual.Data, func(i, j int) bool {
		return actual.Data[i].ObjectName < actual.Data[j].ObjectName
	})

	return reflect.DeepEqual(expected.Data, actual.Data)
}

func assertSecretObjectSlicesEqual(expected, actual []*secretsstorev1.SecretObject) bool {
	if len(expected) != len(actual) {
		return false
	}

	sort.Slice(expected, func(i, j int) bool {
		return expected[i].SecretName < expected[j].SecretName
	})

	sort.Slice(actual, func(i, j int) bool {
		return actual[i].SecretName < actual[j].SecretName
	})

	for i := 0; i < len(expected); i++ {
		if ok := assertSecretsObjectsEqual(expected[i], actual[i]); !ok {
			return false
		}
	}

	return true
}
