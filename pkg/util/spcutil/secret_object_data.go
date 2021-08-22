package spcutil

import (
	"strings"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"

	corev1 "k8s.io/api/core/v1"
)

const (
	tlsKey              = "tls.key"
	tlsCert             = "tls.crt"
	dockerConfigJsonKey = ".dockerconfigjson"
	sshPrivateKey       = "ssh-privatekey"
)

// BuildSecretObjectData builds the .Spec.SecretObjects[*].Data list of a SecretObject when SyncAll is true
func BuildSecretObjectData(files map[string]string, secretObj *v1alpha1.SecretObject) {

	for key := range files {
		nested := strings.Split(key, "/")
		var renamedKey string

		if len(nested) > 0 {
			renamedKey = strings.Join(nested, "-")
		}

		if renamedKey == "" {
			secretObj.Data = append(secretObj.Data, &v1alpha1.SecretObjectData{
				ObjectName: key,
				Key:        key,
			})
			continue
		}

		secretObj.Data = append(secretObj.Data, &v1alpha1.SecretObjectData{
			ObjectName: key,
			Key:        renamedKey,
		})
	}
}

// BuildSecretObjects builds the .Spec.SecretObjects list of a SecretProviderClass when .SyncOptions.SyncAll is true
// How a SecretObject is built is dependent on the type of secret
func BuildSecretObjects(files map[string]string, secretType corev1.SecretType) []*v1alpha1.SecretObject {
	secretObjects := []*v1alpha1.SecretObject{}

	var secretObject *v1alpha1.SecretObject
	for key := range files {

		switch secretType {
		case corev1.SecretTypeOpaque:
			secretObject = createOpaqueSecretDataObject(key)
		case corev1.SecretTypeTLS:
			secretObject = createTLSSecretDataObject(key)
		case corev1.SecretTypeDockerConfigJson:
			secretObject = createDockerConfigJsonSecretDataObject(key)
		case corev1.SecretTypeBasicAuth:
			secretObject = createBasicAuthSecretDataObject(key)
		case corev1.SecretTypeSSHAuth:
			secretObject = createSSHSecretDataObject(key)
		}

		secretObjects = append(secretObjects, secretObject)
	}

	return secretObjects
}

// createOpaqueSecretDataObject creates a SecretObject for an Opaque secret
func createOpaqueSecretDataObject(key string) *v1alpha1.SecretObject {
	return &v1alpha1.SecretObject{
		SecretName: setSecretName(key),
		Type:       string(corev1.SecretTypeOpaque),
		Data: []*v1alpha1.SecretObjectData{
			{
				ObjectName: key,
				Key:        setKey(key),
			},
		},
	}
}

// createTLSSecretDataObject creates a SecretObject for an TLS secret
func createTLSSecretDataObject(key string) *v1alpha1.SecretObject {
	return &v1alpha1.SecretObject{
		SecretName: setSecretName(key),
		Type:       string(corev1.SecretTypeTLS),
		Data: []*v1alpha1.SecretObjectData{
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
func createDockerConfigJsonSecretDataObject(key string) *v1alpha1.SecretObject {
	return &v1alpha1.SecretObject{
		SecretName: setSecretName(key),
		Type:       string(corev1.SecretTypeDockerConfigJson),
		Data: []*v1alpha1.SecretObjectData{
			{
				ObjectName: key,
				Key:        dockerConfigJsonKey,
			},
		},
	}
}

// createBasicAuthSecretDataObject creates a SecretObject for an Basic-Auth secret
func createBasicAuthSecretDataObject(key string) *v1alpha1.SecretObject {
	return &v1alpha1.SecretObject{
		SecretName: setSecretName(key),
		Type:       string(corev1.SecretTypeBasicAuth),
		Data: []*v1alpha1.SecretObjectData{
			{
				ObjectName: key,
				Key:        setKey(key),
			},
		},
	}
}

// createSSHSecretDataObject creates a SecretObject for an SSH-Auth secret
func createSSHSecretDataObject(key string) *v1alpha1.SecretObject {
	return &v1alpha1.SecretObject{
		SecretName: setSecretName(key),
		Type:       string(corev1.SecretTypeSSHAuth),
		Data: []*v1alpha1.SecretObjectData{
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
