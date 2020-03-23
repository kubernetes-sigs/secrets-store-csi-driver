/*
Copyright 2018 The Kubernetes Authors.

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

	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"golang.org/x/net/context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)


var (
	secretProviderClassGvk = schema.GroupVersionKind{
		Group:   "secrets-store.csi.x-k8s.io",
		Version: "v1alpha1",
		Kind:    "SecretProviderClassList",
	}
)


// getProviderPath returns the absolute path to the provider binary
func (ns *nodeServer) getProviderPath(goos string, providerName string) string {
	if goos == "windows" {
		return normalizeWindowsPath(fmt.Sprintf(`%s\%s\provider-%s.exe`, ns.providerVolumePath, providerName, providerName))
	}
	return fmt.Sprintf("%s/%s/provider-%s", ns.providerVolumePath, providerName, providerName)
}

func normalizeWindowsPath(path string) string {
	normalizedPath := strings.Replace(path, "/", "\\", -1)
	if strings.HasPrefix(normalizedPath, "\\") {
		normalizedPath = "c:" + normalizedPath
	}
	return normalizedPath
}

// getMountedFiles returns all the mounted files
func getMountedFiles(targetPath string) ([]string, error) {
	// loop thru all the mounted files
	files, err := filepath.Glob(filepath.Join(targetPath, "*"))
	if err != nil {
		log.Errorf("failed to list all files in target path %s, err: %v", targetPath, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	return files, nil
}

// getPodUIDFromTargetPath returns podUID from targetPath
func getPodUIDFromTargetPath(goos string, targetPath string) (string) {

	var parts []string
	if goos == "windows" {
		if strings.Contains(targetPath, `\var\lib\kubelet\pods`) {
			parts = strings.Split(targetPath, `\`)
		}
	} else {
		if strings.Contains(targetPath, `/var/lib/kubelet/pods`) {
			parts = strings.Split(targetPath, `/`)
		}
	}
	if len(parts) > 6 {
		return parts[5]
	}
	return ""
}

// ensureMountPoint ensures mount point is valid
func (ns *nodeServer) ensureMountPoint(target string) (bool, error) {
	notMnt, err := ns.mounter.IsLikelyNotMountPoint(target)
	if err != nil && !os.IsNotExist(err) {
		return !notMnt, err
	}

	if !notMnt {
		// testing original mount point, make sure the mount link is valid
		_, err := ioutil.ReadDir(target)
		if err == nil {
			log.Infof("already mounted to target %s", target)
			// already mounted
			return !notMnt, nil
		}
		if err := ns.mounter.Unmount(target); err != nil {
			log.Errorf("Unmount directory %s failed with %v", target, err)
			return !notMnt, err
		}
		notMnt = true
		// remount it in node publish
		return !notMnt, err
	}

	if runtime.GOOS == "windows" {
		// IsLikelyNotMountPoint always returns notMnt=true for windows as the
		// target path is not a soft link to the global mount
		// instead check if the dir exists for windows and if it's not empty
		// If there are contents in the dir, then objects are already mounted
		f, err := ioutil.ReadDir(target)
		if err != nil {
			return !notMnt, err
		}
		if len(f) > 0 {
			notMnt = false
			return !notMnt, err
		}
	}

	return false, nil
}

// syncK8sObjects creates or updates K8s secrets based on secretProviderClass spec and data from mounted files in targetPath
// it should also add pod info to the secretProviderClass object's byPod status field
func syncK8sObjects(ctx context.Context, targetPath string, podUID string, namespace string, secretProviderClass string, secretObjects []interface{}) error {
	successfulUpdates := 0
	files, err := getMountedFiles(targetPath)
	if err != nil {
		return err
	}
	for _, s := range secretObjects {
		secretObject, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		secretName, exists, err := unstructured.NestedString(secretObject, "secretName")
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		sType, exists, err := unstructured.NestedString(secretObject, "type")
		if err != nil {
			return err
		}
		if !exists {
			log.Infof("type does not exist in obj")
			continue
		}
		secretType := getSecretType(sType)
		secretObjectDataList, exists, err := unstructured.NestedSlice(secretObject, "data")
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		datamap := make(map[string][]byte)
		for _, d := range secretObjectDataList {
			secretObjectData, ok := d.(map[string]interface{})
			if !ok {
				continue
			}
			objectName, exists, err := unstructured.NestedString(secretObjectData, "objectName")
			if err != nil {
				return err
			}
			if !exists {
				continue
			}
			key, exists, err := unstructured.NestedString(secretObjectData, "key")
			if err != nil {
				return err
			}
			if !exists {
				continue
			}
			found := false
			for _, file := range files {
				filename := filepath.Base(file)
				if filename == objectName {
					found = true
					log.Infof("file matching objectName %s found, processing key %s", objectName, key)
					data, err := ioutil.ReadFile(file)
					if err != nil {
						log.Errorf("failed to read file %s, err: %v", file, err)
						return status.Error(codes.Internal, err.Error())
					}
					if secretType == corev1.SecretTypeTLS {
						data, err = getCertPart(data, key)
						if err != nil {
							log.Errorf("failed to get cert data from file %s, err: %v", file, err)
							return status.Error(codes.Internal, err.Error())
						}
					}
					datamap[key] = data
					break
				}
			}
			if !found {
				log.Errorf("file matching objectName %s not found", objectName)
			}
		}
		createFn := func() (bool, error) {
			if err := createOrUpdateK8sSecret(ctx, secretName, namespace, datamap, secretType); err != nil {
				return false, nil
			}
			successfulUpdates++
			return true, nil
		}
		if err := wait.ExponentialBackoff(wait.Backoff{
			Steps:    5,
			Duration: 1 * time.Millisecond,
			Factor:   1.0,
			Jitter:   0.1,
		}, createFn); err != nil {
			log.Error(err, "max retries for creating secret reached")
			return err
		}
	}
	// only update status when more than one secret has been created
	if successfulUpdates > 0 {
		/// TODO(ritazh): right now assume all files come from one secretproviderclass
		// update instance status field with podUID and namespace
		setStatusFn := func() (bool, error) {
			item, err := getSecretProviderItemByName(ctx, secretProviderClass)
			if err != nil {
				log.Errorf("failed to get secret provider item, err: %v", err)
				return false, nil
			}
			if err := setStatus(ctx, item, podUID, namespace); err != nil {
				log.Errorf("failed to set status, err: %v", err)
				return false, nil
			}
			return true, nil
		}
		if err := wait.ExponentialBackoff(wait.Backoff{
			Steps:    5,
			Duration: 1 * time.Millisecond,
			Factor:   1.0,
			Jitter:   0.1,
		}, setStatusFn); err != nil {
			log.Error(err, "max retries for setting status reached")
			return err
		}
	}
	return nil	
}
// removeK8sObjects deletes K8s secrets based on secretProviderClass spec
// it should also delete pod info from the secretProviderClass object's byPod status field
func removeK8sObjects (ctx context.Context, targetPath string, podUID string, files []string, secretObjects []interface{}) error {
	secretProviderClass := ""
	namespace := ""
	deleteStatusFn := func() (bool, error) {
		item, podNS, err := getItemWithPodID(ctx, podUID)
		if err == nil && len(podNS) > 0 {
			secretProviderClass = item.GetName()
			namespace = podNS
			if err = deleteStatus(ctx, item, podUID); err != nil {
				return false, nil
			}
			return true, nil
		}
		return true, nil
	}

	if err := wait.ExponentialBackoff(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}, deleteStatusFn); err != nil {
		log.Error(err, "max retries for deleting status reached")
		return err
	}

	if len(namespace) > 0 && len(secretProviderClass) > 0 {
		deleteSecretFn := func() (bool, error) {
			item, err := getSecretProviderItemByName(ctx, secretProviderClass)
			if err != nil {
				log.Errorf("failed to get secret provider item, err: %v", err)
				return false, nil
			}
			count := getStatusCount(item)
			// only delete when no more pods are associated with it
			if count == 0 {
				///TODO: we assume all files are mounted from a single secretsproviderclass
				/// a pod could have multiple volumes pointing to diff secretproviderclass objs
				for _, s := range secretObjects {
					secretObject, ok := s.(map[string]interface{})
					if !ok {
						continue
					}
					secretName, exists, err := unstructured.NestedString(secretObject, "secretName")
					if err != nil {
						return false, nil
					}
					if !exists {
						continue
					}
					err = deleteK8sSecret(ctx, secretName, namespace)
					if err != nil {
						return false, nil
					}
				}
			}
			return true, nil
		}

		if err := wait.ExponentialBackoff(wait.Backoff{
			Steps:    5,
			Duration: 1 * time.Millisecond,
			Factor:   1.0,
			Jitter:   0.1,
		}, deleteSecretFn); err != nil {
			log.Error(err, "max retries for deleting secret reached")
			return err
		}
	}
	return nil
}

// createOrUpdateK8sSecret creates or updates a K8s secret with data from mounted files
// If a secret with the same name already exists in the namespace of the pod, it's updated.
func createOrUpdateK8sSecret(ctx context.Context, name string, namespace string, datamap map[string][]byte, secretType corev1.SecretType) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Type:          secretType,
	}

	if err := c.Get(ctx, secretKey, secret); err != nil {
		log.Error(err, "error from c.Get")
		if errors.IsNotFound(err) {
			secret.Data = datamap
			if err := c.Create(ctx, secret); err != nil {
				log.Error(err, "error while creating K8s secret")
				return err
			}
		} else {
			log.Error(err, "error while retrieving K8s secret")
			return err
		}
	} else {
		secret.Data = datamap
		if err := c.Update(ctx, secret); err != nil {
			log.Error(err, "error while updating K8s secret")
			return err
		}
	}
	log.Infof("created k8s secret: %s", name)
	return nil
}

// deleteK8sSecret deletes a secret by name
func deleteK8sSecret(ctx context.Context, name string, namespace string) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	secretKey := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	if err := c.Get(ctx, secretKey, secret); err != nil {
		log.Error(err, "error while getting K8s secret")
		return err
	} 

	if err := c.Delete(ctx, secret); err != nil {
		log.Error(err, "error while deleting K8s secret")
		return err
	}
	
	log.Infof("deleted k8s secret: %s", name)
	return nil
}

// setStatus adds pod-specific info to byPod status of the secretproviderclass object
func setStatus(ctx context.Context, obj *unstructured.Unstructured, id string, namespace string) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	status := map[string]interface{}{
		"id":                  id,
		"namespace":           namespace,
	}
	statuses, exists, err := unstructured.NestedSlice(obj.Object, "status", "byPod")
	if err != nil {
		return err
	}
	if !exists {
		log.Infof("doesnt exist, before set")
		if err := unstructured.SetNestedSlice(
			obj.Object, []interface{}{status}, "status", "byPod"); err != nil {
			return err
		}
		log.Infof("doesnt exist, after set")
	}

	for _, s := range statuses {
		curStatus, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		curID2, ok := curStatus["id"]
		if !ok {
			continue
		}
		curID, ok := curID2.(string)
		if !ok {
			continue
		}
		// skip if id already exists
		if id == curID {
			return nil
		}
	}

	statuses = append(statuses, status)
	if err := unstructured.SetNestedSlice(
		obj.Object, statuses, "status", "byPod"); err != nil {
		return err
	}
	err = c.Update(ctx, obj)
	if err != nil {
		return err
	}
	return nil
}
// deleteStatus deletes pod-specific information from byPod status of the secretproviderclass object
func deleteStatus(ctx context.Context, obj *unstructured.Unstructured, id string) error {
	c, err := getClient()
	if err != nil {
		return err
	}
	statuses, exists, err := unstructured.NestedSlice(obj.Object, "status", "byPod")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	newStatus := make([]interface{}, 0)

	for i, s := range statuses {
		curStatus, ok := s.(map[string]interface{})
		if !ok {
			return fmt.Errorf("element %d in byPod status is malformed", i)
		}
		curID2, ok := curStatus["id"]
		if !ok {
			return fmt.Errorf("element %d in byPod status is missing an `id` field", i)
		}
		curID, ok := curID2.(string)
		if !ok {
			return fmt.Errorf("element %d in byPod status' `id` field is not a string: %v", i, curID2)
		}
		if id == curID {
			continue
		}
		newStatus = append(newStatus, s)
	}
	if err := unstructured.SetNestedSlice(obj.Object, newStatus, "status", "byPod"); err != nil {
		return err
	}
	err = c.Update(ctx, obj)
	if err != nil {
		return err
	}
	return nil
}
// getNamespaceByPodID returns namespace of the pod with podUID from the status of the secretproviderclass object
func getNamespaceByPodID(obj *unstructured.Unstructured, id string) (string, error) {
	statuses, exists, err := unstructured.NestedSlice(obj.Object, "status", "byPod")
	if err != nil {
		return "", err
	}
	if !exists {
		return "", nil
	}

	for i, s := range statuses {
		curStatus, ok := s.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("element %d in byPod status is malformed", i)
		}
		curID2, ok := curStatus["id"]
		if !ok {
			return "", fmt.Errorf("element %d in byPod status is missing an `id` field", i)
		}
		curID, ok := curID2.(string)
		if !ok {
			return "", fmt.Errorf("element %d in byPod status' `id` field is not a string: %v", i, curID2)
		}
		if id == curID {
			namespace2, ok := curStatus["namespace"]
			if !ok {
				return "", fmt.Errorf("element %d in byPod status is missing an `namespace` field", i)
			}
			namespace, ok := namespace2.(string)
			if !ok {
				return "", fmt.Errorf("element %d in byPod status' `namespace` field is not a string: %v", i, namespace2)
			}
			return namespace, nil
		}
	}

	return "", fmt.Errorf("could not find id %s in status", id)
}

// getClient returns client.Client
func getClient() (client.Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	c, err := client.New(cfg, client.Options{Scheme: nil, Mapper: nil})
	if err != nil {
		return nil, err
	}
	return c, nil
}
// getStatusCount returns the total number of objects in the byPod status field of the secretproviderclass object
func getStatusCount(obj *unstructured.Unstructured) int {
	statuses, exists, err := unstructured.NestedSlice(obj.Object, "status", "byPod")
	if err == nil && exists {
		return len(statuses)
	}
	return 0
}
// getSecretProviderItemByName returns the secretproviderclass object by name
func getSecretProviderItemByName (ctx context.Context, name string) (*unstructured.Unstructured, error) {
	instanceList := &unstructured.UnstructuredList{}
	instanceList.SetGroupVersionKind(secretProviderClassGvk)
	c, err := getClient()
	if err != nil {
		return nil, err
	}
	err = c.List(ctx, instanceList)
	if err != nil {
		return nil, err
	}

	for _, item := range instanceList.Items {
		if item.GetName() == name {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("could not find secretproviderclass %s", name)
}
// getItemWithPodID returns the secretproviderclass object with podUID 
func getItemWithPodID (ctx context.Context, podUID string) (*unstructured.Unstructured, string, error) {
	c, err := getClient()
	if err != nil {
		return nil, "", err
	}
	instanceList := &unstructured.UnstructuredList{}
	instanceList.SetGroupVersionKind(secretProviderClassGvk)
	err = c.List(ctx, instanceList)
	if err != nil {
		return nil, "", err
	}

	for _, item := range instanceList.Items {
		podNS, err := getNamespaceByPodID(&item, podUID)
		if err != nil || len(podNS) == 0 {
			continue
		}
		if err == nil && len(podNS) > 0 {
			return &item, podNS, nil
		}
	}
	return nil, "", nil
}
// getSecretObjectsFromSpec returns secretObjects and if it exists in the spec
func getSecretObjectsFromSpec (item *unstructured.Unstructured) ([]interface{}, bool, error) {
	secretObjects, exists, err := unstructured.NestedSlice(item.Object, "spec", "secretObjects")
	if err != nil {
		return nil, false, err
	}
	return secretObjects, exists && len(secretObjects) > 0, nil
}
// getSecretType returns a k8s secret type, defaults to Opaque
func getSecretType (sType string) (corev1.SecretType){
	switch sType {
	case "kubernetes.io/basic-auth":
		return corev1.SecretTypeBasicAuth
	case "bootstrap.kubernetes.io/token":
		return corev1.SecretTypeBootstrapToken
	case "kubernetes.io/dockerconfigjson":
		return corev1.SecretTypeDockerConfigJson
	case "kubernetes.io/dockercfg":
		return corev1.SecretTypeDockercfg
	case "kubernetes.io/ssh-auth":
		return corev1.SecretTypeSSHAuth
	case "kubernetes.io/service-account-token":
		return corev1.SecretTypeServiceAccountToken
	case "kubernetes.io/tls":
		return corev1.SecretTypeTLS
	default:
		return corev1.SecretTypeOpaque
	}
}
// getCertPart returns the certificate or the private key part of the cert
func getCertPart(data []byte, key string) ([]byte, error) {
	if key == corev1.TLSPrivateKeyKey {
		return getPrivateKey(data)
	}
	if key == corev1.TLSCertKey {
		return getCert(data)
	}
	return nil, fmt.Errorf("tls key is not supported. Only tls.key and tls.crt are supported")
}
// getCert returns the certificate part of a cert
func getCert(data []byte) ([]byte, error) {
	var certs []byte
	for {
		pemBlock, rest := pem.Decode(data)
		if pemBlock == nil {
			break
		}
		if pemBlock.Type == "CERTIFICATE" {
			block := pem.EncodeToMemory(pemBlock)
			certs = append(certs, block...)
		}
		data = rest
	}
	return certs, nil
}
// getPrivateKey returns the private key part of a cert
func getPrivateKey(data []byte) ([]byte, error) {
	var der []byte
	var derKey []byte
	for {
		pemBlock, rest := pem.Decode(data)
		if pemBlock == nil {
			break
		}
		if pemBlock.Type != "CERTIFICATE" {
			der = pemBlock.Bytes
		}
		data = rest
	}

	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		derKey = x509.MarshalPKCS1PrivateKey(key)
	}

	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey:
			derKey = x509.MarshalPKCS1PrivateKey(key)
		case *ecdsa.PrivateKey:
			derKey, err = x509.MarshalECPrivateKey(key)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown private key type found while getting key. Only rsa and ecdsa are supported")
		}
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		derKey, err = x509.MarshalECPrivateKey(key)
		if err != nil {
			return nil, err
		}
	}
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: derKey,
	}
	
	return pem.EncodeToMemory(block), nil
}
