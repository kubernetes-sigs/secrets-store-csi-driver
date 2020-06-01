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
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
)

var (
	secretProviderClassGvk = schema.GroupVersionKind{
		Group:   "secrets-store.csi.x-k8s.io",
		Version: "v1alpha1",
		Kind:    "SecretProviderClassList",
	}
)

const (
	secretNameField    = "secretName"
	secretObjectsField = "secretObjects"
	keyField           = "key"
	dataField          = "data"
	objectNameField    = "objectName"
	typeField          = "type"
	certType           = "CERTIFICATE"
	privateKeyType     = "RSA PRIVATE KEY"
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

// getMountedFiles returns all the mounted files names
func getMountedFiles(targetPath string) ([]string, error) {
	var paths []string
	// loop thru all the mounted files
	files, err := ioutil.ReadDir(targetPath)
	if err != nil {
		log.Errorf("failed to list all files in target path %s, err: %v", targetPath, err)
		return nil,
			status.Error(codes.Internal, err.Error())
	}
	sep := "/"
	if strings.HasPrefix(targetPath, "c:\\") {
		sep = "\\"
	} else if strings.HasPrefix(targetPath, `c:\`) {
		sep = `\`
	}
	for _, file := range files {
		paths = append(paths, targetPath+sep+file.Name())
	}
	return paths, nil
}

// getPodUIDFromTargetPath returns podUID from targetPath
func getPodUIDFromTargetPath(targetPath string) string {
	re := regexp.MustCompile(`[\\|\/]+pods[\\|\/]+(.+?)[\\|\/]+volumes`)
	match := re.FindStringSubmatch(targetPath)
	if len(match) < 2 {
		return ""
	}
	return match[1]
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
func syncK8sObjects(ctx context.Context, targetPath, podUID, namespace, secretProviderClass, provider string, secretObjects []interface{}, reporter StatsReporter) error {
	successfulUpdates := 0
	files, err := getMountedFiles(targetPath)
	if err != nil {
		return err
	}
	for _, s := range secretObjects {
		secretObject, ok := s.(map[string]interface{})
		if !ok {
			log.Infof("could not cast secretObject as map[string]interface{} for pod: %s, ns: %s", podUID, namespace)
			continue
		}
		secretName, err := getStringFromObject(secretObject, secretNameField)
		if err != nil {
			log.Infof("could not get secretName from secretObject for pod: %s, ns: %s", podUID, namespace)
			continue
		}
		sType, err := getStringFromObject(secretObject, typeField)
		if err != nil {
			log.Infof("could not get type from secretObject for pod: %s, ns: %s", podUID, namespace)
			continue
		}
		secretType := getSecretType(sType)
		secretObjectDataList, err := getSliceFromObject(secretObject, dataField)
		if err != nil {
			log.Infof("could not get data from secretObject for pod: %s, ns: %s", podUID, namespace)
			continue
		}
		datamap := make(map[string][]byte)
		for _, d := range secretObjectDataList {
			secretObjectData, ok := d.(map[string]interface{})
			if !ok {
				log.Infof("could not cast secretObject data as map[string]interface{} for pod: %s, ns: %s", podUID, namespace)
				continue
			}
			objectName, err := getStringFromObject(secretObjectData, objectNameField)
			if err != nil {
				log.Infof("could not get objectName from secretObject data for pod: %s, ns: %s", podUID, namespace)
				continue
			}
			key, err := getStringFromObject(secretObjectData, keyField)
			if err != nil {
				log.Infof("could not get key from secretObject data for pod: %s, ns: %s", podUID, namespace)
				continue
			}
			found := false
			for _, file := range files {
				filename := filepath.Base(file)
				if filename == objectName {
					found = true
					log.Infof("file matching objectName %s found, processing key %s for pod: %s, ns: %s", objectName, key, podUID, namespace)
					data, err := ioutil.ReadFile(file)
					if err != nil {
						log.Errorf("failed to read file %s, err: %v for pod: %s, ns: %s", file, err, podUID, namespace)
						return status.Error(codes.Internal, err.Error())
					}
					if secretType == corev1.SecretTypeTLS {
						data, err = getCertPart(data, key)
						if err != nil {
							log.Errorf("failed to get cert data from file %s, err: %v for pod: %s, ns: %s", file, err, podUID, namespace)
							return status.Error(codes.Internal, err.Error())
						}
					}
					datamap[key] = data
					break
				}
			}
			if !found {
				log.Errorf("file matching objectName %s not found for pod: %s, ns: %s", objectName, podUID, namespace)
			}
		}
		createFn := func() (bool, error) {
			if err := createOrUpdateK8sSecret(ctx, secretName, namespace, datamap, secretType); err != nil {
				log.Errorf("failed createOrUpdateK8sSecret, err: %v for pod: %s, ns: %s", err, podUID, namespace)
				return false, nil
			}
			successfulUpdates++
			return true, nil
		}
		begin := time.Now()
		if err := wait.ExponentialBackoff(wait.Backoff{
			Steps:    5,
			Duration: 1 * time.Millisecond,
			Factor:   1.0,
			Jitter:   0.1,
		}, createFn); err != nil {
			log.Errorf("max retries for creating secret reached, err: %v for pod: %s, ns: %s", err, podUID, namespace)
			return err
		}
		reporter.reportSyncK8SecretDuration(time.Since(begin).Seconds())
	}
	reporter.reportSyncK8SecretCtMetric(provider, successfulUpdates)

	// only update status when more than one secret has been created
	if successfulUpdates > 0 {
		/// TODO(ritazh): right now assume all files come from one secretproviderclass
		// update instance status field with podUID and namespace
		setStatusFn := func() (bool, error) {
			item, err := getSecretProviderItemByName(ctx, secretProviderClass)
			if err != nil {
				log.Errorf("failed to get secret provider item, err: %v for pod: %s, ns: %s", err, podUID, namespace)
				return false, nil
			}
			if err := setStatus(ctx, item, podUID, namespace); err != nil {
				log.Errorf("failed to set status, err: %v for pod: %s, ns: %s", err, podUID, namespace)
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
			log.Errorf("max retries for setting status reached, err: %v for pod: %s, ns: %s", err, podUID, namespace)
			return err
		}
	}
	return nil
}

// removeK8sObjects deletes K8s secrets based on secretProviderClass spec
// it should also delete pod info from the secretProviderClass object's byPod status field
func removeK8sObjects(ctx context.Context, targetPath string, podUID string, files []string, secretObjects []interface{}) error {
	var secretProviderClass, namespace string

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
		log.Errorf("max retries for deleting status reached, err: %v for pod: %s, ns: %s", err, podUID, namespace)
		return err
	}

	if len(namespace) > 0 && len(secretProviderClass) > 0 {
		deleteSecretFn := func() (bool, error) {
			item, err := getSecretProviderItemByName(ctx, secretProviderClass)
			if err != nil {
				log.Errorf("failed to get secret provider item, err: %v for pod: %s, ns: %s", err, podUID, namespace)
				return false, nil
			}
			count, err := getStatusCount(item)
			if err != nil {
				log.Errorf("failed to get status count, err: %v for object: %s", err, item.GetName())
				return false, nil
			}
			// only delete when no more pods are associated with it
			if count == 0 {
				///TODO: we assume all files are mounted from a single secretsproviderclass
				/// a pod could have multiple volumes pointing to diff secretproviderclass objs
				for _, s := range secretObjects {
					secretObject, ok := s.(map[string]interface{})
					if !ok {
						continue
					}

					secretName, err := getStringFromObject(secretObject, secretNameField)
					if err != nil {
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
			log.Errorf("max retries for deleting secret reached, err: %v for pod: %s, ns: %s", err, podUID, namespace)
			return err
		}
	}
	return nil
}

// createOrUpdateK8sSecret creates or updates a K8s secret with data from mounted files
// If a secret with the same name already exists in the namespace of the pod, it's updated.
func createOrUpdateK8sSecret(ctx context.Context, name string, namespace string, datamap map[string][]byte, secretType corev1.SecretType) error {
	// recreating client here to prevent reading from cache
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
		Type: secretType,
	}

	err = c.Get(ctx, secretKey, secret)
	if err != nil {
		log.Errorf("error from c.Get: %v for secret: %s, ns: %s", err, name, namespace)
		if errors.IsNotFound(err) {
			secret.Data = datamap
			if err := c.Create(ctx, secret); err != nil {
				log.Errorf("error %v while creating K8s secret: %s, ns: %s", err, name, namespace)
				return err
			}
			return nil
		}
		log.Errorf("error %v while retrieving K8s secret: %s, ns: %s", err, name, namespace)
		return err
	}
	secret.Data = datamap
	if err := c.Update(ctx, secret); err != nil {
		log.Errorf("error %v while updating K8s secret: %s, ns: %s", err, name, namespace)
		return err
	}

	log.Infof("created k8s secret: %s, ns: %s", name, namespace)
	return nil
}

// deleteK8sSecret deletes a secret by name
func deleteK8sSecret(ctx context.Context, name string, namespace string) error {
	// recreating client here to prevent reading from cache
	c, err := getClient()
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	if err := c.Delete(ctx, secret); err != nil {
		if errors.IsNotFound(err) {
			log.Infof("k8s secret not found during delete. Skip. secret: %s, ns: %s", name, namespace)
			return nil
		}
		log.Errorf("error %v while deleting K8s secret: %s, ns: %s", err, name, namespace)
		return err
	}

	log.Infof("deleted k8s secret: %s, ns: %s", name, namespace)
	return nil
}

// setStatus adds pod-specific info to byPod status of the secretproviderclass object
func setStatus(ctx context.Context, obj *unstructured.Unstructured, id string, namespace string) error {
	log.Infof("setStatus for pod: %s, ns: %s", id, namespace)
	// recreating client here to prevent reading from cache
	c, err := getClient()
	if err != nil {
		return err
	}
	status := map[string]interface{}{
		"id":        id,
		"namespace": namespace,
	}
	statuses, _, err := unstructured.NestedSlice(obj.Object, "status", "byPod")
	if err != nil {
		return err
	}

	for _, s := range statuses {
		curStatus, ok := s.(map[string]interface{})
		if !ok {
			log.Infof("could not cast status as map[string]interface{} for object: %s", obj.GetName())
			continue
		}
		curID2, ok := curStatus["id"]
		if !ok {
			log.Infof("could not get id from status for object: %s", obj.GetName())
			continue
		}
		curID, ok := curID2.(string)
		if !ok {
			log.Infof("could not cast id from status as string for object: %s", obj.GetName())
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
	// recreating client here to prevent reading from cache
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
			return fmt.Errorf("element %d in byPod status is malformed for object: %s", i, obj.GetName())
		}
		curID2, ok := curStatus["id"]
		if !ok {
			return fmt.Errorf("element %d in byPod status is missing an `id` field for object: %s", i, obj.GetName())
		}
		curID, ok := curID2.(string)
		if !ok {
			return fmt.Errorf("element %d in byPod status' `id` field is not a string: %v for object: %s", i, curID2, obj.GetName())
		}
		if id == curID {
			continue
		}
		newStatus = append(newStatus, s)
	}

	if len(newStatus) == len(statuses) {
		log.Infof("could not find pod %s in status for object: %s. Skip updating object", id, obj.GetName())
		return nil
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
			return "", fmt.Errorf("element %d in byPod status is malformed for pod: %s", i, id)
		}
		curID2, ok := curStatus["id"]
		if !ok {
			return "", fmt.Errorf("element %d in byPod status is missing an `id` field for pod: %s", i, id)
		}
		curID, ok := curID2.(string)
		if !ok {
			return "", fmt.Errorf("element %d in byPod status' `id` field is not a string: %v for pod: %s", i, curID2, id)
		}
		if id == curID {
			namespace2, ok := curStatus["namespace"]
			if !ok {
				return "", fmt.Errorf("element %d in byPod status is missing an `namespace` field for pod: %s", i, id)
			}
			namespace, ok := namespace2.(string)
			if !ok {
				return "", fmt.Errorf("element %d in byPod status' `namespace` field is not a string: %v for pod: %s", i, namespace2, id)
			}
			return namespace, nil
		}
	}

	return "", fmt.Errorf("could not find pod id %s in status", id)
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
func getStatusCount(obj *unstructured.Unstructured) (int, error) {
	statuses, exists, err := unstructured.NestedSlice(obj.Object, "status", "byPod")
	if err != nil {
		return 0, err
	}
	if err == nil && exists {
		return len(statuses), nil
	}
	return 0, nil
}

// getSecretProviderItemByName returns the secretproviderclass object by name
func getSecretProviderItemByName(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	instanceList := &unstructured.UnstructuredList{}
	instanceList.SetGroupVersionKind(secretProviderClassGvk)
	// recreating client here to prevent reading from cache
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
func getItemWithPodID(ctx context.Context, podUID string) (*unstructured.Unstructured, string, error) {
	// recreating client here to prevent reading from cache
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

// getSecretObjectsFromSpec returns secretObjects if it exists in the spec
func getSecretObjectsFromSpec(item *unstructured.Unstructured) ([]interface{}, bool, error) {
	secretObjects, exists, err := unstructured.NestedSlice(item.Object, "spec", secretObjectsField)
	if err != nil {
		return nil, false, err
	}
	return secretObjects, exists && len(secretObjects) > 0, nil
}

// getSecretType returns a k8s secret type, defaults to Opaque
func getSecretType(sType string) corev1.SecretType {
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

func getStringFromObjectSpec(object map[string]interface{}, key string) (string, error) {
	value, exists, err := unstructured.NestedString(object, "spec", key)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("could not get field %s from spec", key)
	}
	if len(value) == 0 {
		return "", fmt.Errorf("field %s is not set", key)
	}
	return value, nil
}

func getStringFromObject(object map[string]interface{}, key string) (string, error) {
	value, exists, err := unstructured.NestedString(object, key)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("could not get field %s from spec", key)
	}
	if len(value) == 0 {
		return "", fmt.Errorf("field %s is not set", key)
	}
	return value, nil
}

func getMapFromObjectSpec(object map[string]interface{}, key string) (map[string]string, error) {
	value, exists, err := unstructured.NestedStringMap(object, "spec", key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("could not get field %s from spec", key)
	}
	if len(value) == 0 {
		return nil, fmt.Errorf("field %s is not set", key)
	}
	return value, nil
}

func getSliceFromObject(object map[string]interface{}, key string) ([]interface{}, error) {
	value, exists, err := unstructured.NestedSlice(object, key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("could not get field %s from spec", key)
	}
	if len(value) == 0 {
		return nil, fmt.Errorf("field %s is not set", key)
	}
	return value, nil
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
		if pemBlock.Type == certType {
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
		if pemBlock.Type != certType {
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
		Type:  privateKeyType,
		Bytes: derKey,
	}

	return pem.EncodeToMemory(block), nil
}

func createSecretProviderClassPodStatus(ctx context.Context, podname, namespace, podUID, spcName, targetPath, nodeID string, mounted bool) error {
	obj := &unstructured.Unstructured{}
	obj.SetName(podname + "-" + namespace + "-" + spcName)
	obj.SetNamespace(namespace)
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v1alpha1.GroupVersion.Group,
		Version: v1alpha1.GroupVersion.Version,
		Kind:    "SecretProviderClassPodStatus",
	})
	// Set owner reference to the pod as the mapping between secret provider class pod status and
	// pod is 1 to 1. When pod is deleted, the spc pod status will automatically garbage collected
	obj.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: "v1",
			Kind:       "Pod",
			Name:       podname,
			UID:        types.UID(podUID),
		},
	})

	status := map[string]interface{}{
		"podName":                 podname,
		"targetPath":              targetPath,
		"mounted":                 mounted,
		"secretProviderClassName": spcName,
	}

	if err := unstructured.SetNestedField(
		obj.Object, status, "status"); err != nil {
		return err
	}

	obj.SetLabels(map[string]string{
		"internal.secrets-store.csi.k8s.io/node-name": nodeID,
	})
	// recreating client here to prevent reading from cache
	c, err := getClient()
	if err != nil {
		return err
	}
	// create the secret provider class pod status
	err = c.Create(ctx, obj, &client.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
