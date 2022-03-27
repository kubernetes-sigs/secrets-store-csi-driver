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
	"strings"

	corev1 "k8s.io/api/core/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

const (
	tlsKey              = "tls.key"
	tlsCert             = "tls.crt"
	dockerConfigJsonKey = ".dockerconfigjson"
	sshPrivateKey       = "ssh-privatekey"
)

// BuildSecretObjects builds the .Spec.SecretObjects list of a SecretProviderClass when .SyncOptions.SyncAll is true
// How a SecretObject is built is dependent on the type of secret
func BuildSecretObjects(files map[string]string, secretType corev1.SecretType /*, format string*/) []*secretsstorev1.SecretObject {
	secretObjects := make([]*secretsstorev1.SecretObject, 0)
	for key := range files {

		switch secretType {
		case corev1.SecretTypeOpaque:
			secretObjects = append(secretObjects, createOpaqueSecretDataObject(key))
		case corev1.SecretTypeTLS:
			secretObjects = append(secretObjects, createTLSSecretDataObject(key))
		case corev1.SecretTypeDockerConfigJson:
			secretObjects = append(secretObjects, createDockerConfigJsonSecretDataObject(key))
		case corev1.SecretTypeBasicAuth:
			secretObjects = append(secretObjects, createBasicAuthSecretDataObject(key))
		case corev1.SecretTypeSSHAuth:
			secretObjects = append(secretObjects, createSSHSecretDataObject(key))
		}
	}

	return secretObjects
}

// createOpaqueSecretDataObject creates a SecretObject for an Opaque secret
func createOpaqueSecretDataObject(key string) *secretsstorev1.SecretObject {
	return &secretsstorev1.SecretObject{
		SecretName: setSecretName(key),
		Type:       string(corev1.SecretTypeOpaque),
		Data: []*secretsstorev1.SecretObjectData{
			{
				ObjectName: key,
				Key:        setKey(key),
			},
		},
	}
}

// createTLSSecretDataObject creates a SecretObject for an TLS secret
func createTLSSecretDataObject(key string) *secretsstorev1.SecretObject {
	return &secretsstorev1.SecretObject{
		SecretName: setSecretName(key),
		Type:       string(corev1.SecretTypeTLS),
		Data: []*secretsstorev1.SecretObjectData{
			{
				ObjectName: key,
				Key:        tlsKey,
			},
			{
				ObjectName: key,
				Key:        tlsCert,
			},
		},
	}
}

// createDockerConfigJsonSecretDataObject creates a SecretObject for an DockerConfigJSON secret
func createDockerConfigJsonSecretDataObject(key string) *secretsstorev1.SecretObject {
	return &secretsstorev1.SecretObject{
		SecretName: setSecretName(key),
		Type:       string(corev1.SecretTypeDockerConfigJson),
		Data: []*secretsstorev1.SecretObjectData{
			{
				ObjectName: key,
				Key:        dockerConfigJsonKey,
			},
		},
	}
}

// createBasicAuthSecretDataObject creates a SecretObject for an Basic-Auth secret
func createBasicAuthSecretDataObject(key string) *secretsstorev1.SecretObject {
	return &secretsstorev1.SecretObject{
		SecretName: setSecretName(key),
		Type:       string(corev1.SecretTypeBasicAuth),
		Data: []*secretsstorev1.SecretObjectData{
			{
				ObjectName: key,
				Key:        setKey(key),
			},
		},
	}
}

// createSSHSecretDataObject creates a SecretObject for an SSH-Auth secret
func createSSHSecretDataObject(key string) *secretsstorev1.SecretObject {
	return &secretsstorev1.SecretObject{
		SecretName: setSecretName(key),
		Type:       string(corev1.SecretTypeSSHAuth),
		Data: []*secretsstorev1.SecretObjectData{
			{
				ObjectName: key,
				Key:        sshPrivateKey,
			},
		},
	}
}

// setSecretName sets the name of a secret to the value of "objectName" separated by "-"
func setSecretName(key string) string {
	nested := strings.Split(key, "/")

	if len(nested) > 0 {
		return strings.Join(nested, "-")
	}

	return key
}

// setKey sets the key of a secret to the name of the mounted file
func setKey(key string) string {
	nested := strings.Split(key, "/")

	if len(nested) > 0 {
		return nested[len(nested)-1]
	}

	return key
}
