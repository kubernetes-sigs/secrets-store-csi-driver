package spcutil

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/secretutil"
)

const (
	tlsKey              = "tls.key"
	tlsCert             = "tls.crt"
	dockerConfigJsonKey = ".dockerconfigjson"
	basicAuthUsername   = "username"
	basicAuthPassword   = "password"
)

// builds the data field of a SecretObject when syncAll is true
func BuildSecretObjectData(files map[string]string, secretObj *v1alpha1.SecretObject) {

	for key := range files {
		nested := strings.Split(key, "/")
		var renamedKey string

		if len(nested) > 0 {
			renamedKey = strings.Join(nested, "_")
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

func BuildSecretObjects(files map[string]string, secretType string) []*v1alpha1.SecretObject {
	secretObjects := []*v1alpha1.SecretObject{}

	var secretObject *v1alpha1.SecretObject
	for key := range files {

		switch {
		case secretutil.GetSecretType(strings.TrimSpace(secretType)) == corev1.SecretTypeOpaque:
			secretObject = buildOpaqueSecretDataObject(key, secretType)
		case secretutil.GetSecretType(strings.TrimSpace(secretType)) == corev1.SecretTypeTLS:
			secretObject = buildTLSSecretDataObject(key, secretType)
		case secretutil.GetSecretType(strings.TrimSpace(secretType)) == corev1.SecretTypeDockerConfigJson:
			secretObject = buildDockerConfigJsonSecretDataObject(key, secretType)
		case secretutil.GetSecretType(strings.TrimSpace(secretType)) == corev1.SecretTypeBasicAuth:
			secretObject = buildBasicAuthSecretDataObject(key, secretType)
		}

		secretObjects = append(secretObjects, secretObject)
	}

	return secretObjects
}

func buildOpaqueSecretDataObject(key string, secretType string) *v1alpha1.SecretObject {
	return &v1alpha1.SecretObject{
		SecretName: key,
		Type:       secretType,
		Data: []*v1alpha1.SecretObjectData{
			{
				ObjectName: key,
				Key:        key,
			},
		},
	}
}

func buildTLSSecretDataObject(key string, secretType string) *v1alpha1.SecretObject {
	return &v1alpha1.SecretObject{
		SecretName: key,
		Type:       secretType,
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

func buildDockerConfigJsonSecretDataObject(key string, secretType string) *v1alpha1.SecretObject {
	return &v1alpha1.SecretObject{
		SecretName: key,
		Type:       secretType,
		Data: []*v1alpha1.SecretObjectData{
			{
				ObjectName: key,
				Key:        dockerConfigJsonKey,
			},
		},
	}
}

func buildBasicAuthSecretDataObject(key string, secretType string) *v1alpha1.SecretObject {

	return &v1alpha1.SecretObject{
		SecretName: key,
		Type:       secretType,
		Data: []*v1alpha1.SecretObjectData{
			{
				ObjectName: key,
				Key:        key,
			},
		},
	}
}
