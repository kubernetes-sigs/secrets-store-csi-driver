package spcutil

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
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

// BuildSecretObjects build the .Spec.SecretObjects list of a SecretProviderClass with .SyncOptions.SyncAll is true
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
