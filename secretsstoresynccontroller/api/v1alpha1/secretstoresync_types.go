/*
Copyright 2023 The Kubernetes Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretObjectData defines the desired state of synchronized data within a Kubernetes secret object.
type SecretObjectData struct {
	// SecretDataValueSource is the data source value of the secret defined in the Secret Provider Class.
	// +kubebuilder:validation:Required
	SecretDataValueSource string `json:"secretDataValueSource"`

	// SecretDataKey is the key in the Kubernetes secret's data field as described in the Kubernetes API reference:
	// https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/secret-v1/
	// +kubebuilder:validation:Required
	SecretDataKey string `json:"secretDataKey"`
}

// SecretObject defines the desired state of synchronized Kubernetes secret objects.
type SecretObject struct {
	// Type specifies the type of the Kubernetes secret object, e.g., Opaque. The controller doesn't have permissions
	// to create a secret object with other types than the ones specified in the helm chart:
	// e.g. "Opaque";"kubernetes.io/basic-auth";"kubernetes.io/ssh-auth";"kubernetes.io/tls"
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// Data is a slice of SecretObjectData containing secret data source from the Secret Provider Class and the
	// corresponding data field key used in the Kubernetes secret object.
	// +kubebuilder:validation:Required
	Data []SecretObjectData `json:"data"`

	// Labels contains key-value pairs representing labels associated with the Kubernetes secret object.
	// The labels are used to identify the secret object.
	// On secret creation, the following label is added: secrets-store.sync.x-k8s.io/secretsync=<secret-sync-name>.
	// Creation fails if the label is specified in the SecretSync object with a different value.
	// On secret update, if the validation admission policy is set, the controller will check if the label
	// secrets-store.sync.x-k8s.io/secretsync=<secret-sync-name> is present. If the label is not present,
	// controller fails to update the secret.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations contains key-value pairs representing annotations associated with the Kubernetes secret object.
	// The following annotation prefix is reserved: secrets-store.sync.x-k8s.io/.
	// Creation fails if the annotation is specified in the SecretsStore object by the user.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// SecretSyncSpec defines the desired state for synchronizing secret.
type SecretSyncSpec struct {
	// SecretSyncControllerName specifies the name of the secret sync controller used to synchronize
	// the secret.
	// +kubebuilder:default:=""
	SecretSyncControllerName string `json:"secretSyncControllerName"`

	// SecretProviderClassName specifies the name of the secret provider class used to pass information to
	// access the secret store.
	// +kubebuilder:validation:Required
	SecretProviderClassName string `json:"secretProviderClassName"`

	// ServiceAccountName specifies the name of the service account used to access the secret store.
	// +kubebuilder:validation:Required
	ServiceAccountName string `json:"serviceAccountName"`

	// SecretObject specifies the configuration for the synchronized Kubernetes secret object.
	// +kubebuilder:validation:Required
	SecretObject SecretObject `json:"secretObject"`

	// ForceSynchronization can be used to force the secret synchronization.
	// This provides a mechanism to trigger a secret synchronization, for example if the secret hash is the same and
	// the user requires a secret update. The string is not used for any other purpose than to trigger a secret
	// synchronization.
	// This field is not used to resolve synchronization conflicts.
	// It is not related with the force query parameter in the Apply operation.
	// https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
	// +optional
	ForceSynchronization string `json:"forceSynchronization,omitempty"`
}

// SecretSyncStatus defines the observed state of the secret synchronization process.
type SecretSyncStatus struct {
	// SecretObjectHash contains the hash of the secret object data, data from the SecretProviderClass (e.g. UID,
	// apiversion, name, namespace, parameters), and similar data from the SecretSync. This hash is used to
	// determine if the secret changed.
	// The hash is calculated using the HMAC (Hash-based Message Authentication Code) algorithm, using bcrypt
	// hashing with the SecretsStoreSync's UID as the key.
	// 1. If the hash is different, the secret is updated.
	// 2. If the hash is the same, the secret is still updated when:
	//		1. The LastRetrievedTimestamp is older than the current time minus the
	//			rotationPollInterval, the secret is updated.
	// 		2. The ForceSynchronization is set, the secret is updated.
	//		3. The SecretUpdateStatus is 'Failed', the secret is updated.
	// +optional
	SecretDataObjectHash string `json:"secretDataObjectHash,omitempty"`

	// LastRetrievedTimestamp represents the last time the secret was retrieved from the Provider and updated.
	// +optional
	LastRetrievedTimestamp *metav1.Time `json:"lastRetrievedTimestamp,omitempty"`

	// SecretUpdateStatus represents the status of the secret update process. The status is set to Succeeded
	// if the secret was created or updated successfully. The status is set to Failed if the secret create
	// or update failed.
	// +optional
	// +kubebuilder:validation:example={secretUpdateStatus:"Succeeded",secretUpdateStatus:"Failed"}
	SecretUpdateStatus string `json:"secretUpdateStatus,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status

// SecretSync represents the desired state and observed state of the secret synchronization process.
type SecretSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretSyncSpec   `json:"spec,omitempty"`
	Status SecretSyncStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SecretSyncList contains a list of SecretSync resources.
type SecretSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretSync `json:"items"`
}

func init() {
	// SchemeBuilder.Register(&SecretSync{}, &SecretSyncList{})
}
