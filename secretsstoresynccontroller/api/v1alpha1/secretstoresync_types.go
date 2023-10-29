/*
Copyright 2023.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SecretObjectData defines the desired state of synchronized data within a Kubernetes secret object.
type SecretObjectData struct {
	// ObjectName is the name of the secret defined in the secret store. It must be either the secret name or
	// the secret alias defined in the Secret Provider Class. This field is required.
	ObjectName string `json:"objectName"`

	// ObjectDataFieldKey is the key in the Kubernetes secret's data field map. This field is required.
	ObjectDataFieldKey string `json:"objectDataKey"`
}

// SecretObject defines the desired state of synchronized Kubernetes secret objects.
type SecretObject struct {
	// Type specifies the type of the Kubernetes secret object, e.g., Opaque.
	// This field is required.
	Type string `json:"type"`

	// Data is a slice of SecretObjectData containing the object name (or alias) and the corresponding data field key
	// used in the Kubernetes secret object. This field is required.
	Data []SecretObjectData `json:"data"`

	// Labels contains key-value pairs representing labels associated with the Kubernetes secret object.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations contains key-value pairs representing annotations associated with the Kubernetes secret object.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// SecretStoreSyncSpec defines the desired state for synchronizing secret.
type SecretStoreSyncSpec struct {
	// SecretProviderClassName specifies the name of the secret provider class used to pass information to
	// access the secret store. This field is required.
	SecretProviderClassName string `json:"secretProviderClassName"`

	// ServiceAccountName specifies the name of the service account used to access the secret store.
	// This field is required.
	ServiceAccountName string `json:"serviceAccountName"`

	// SecretObject specifies the configuration for the synchronized Kubernetes secret object.
	// This field is required.
	SecretObject SecretObject `json:"secretObject"`

	// ForceSynchronization, if set to true, forces the controller to synchronize the secret.
	// +optional
	ForceSynchronization bool `json:"forceSynchronization,omitempty"`
}

// SecretStoreSyncStatus defines the observed state of the secret synchronization process.
type SecretStoreSyncStatus struct {
	// SecretObjectHash contains the hash of the secret object data, used to determine if the data has changed.
	// +optional
	SecretDataObjectHash string `json:"secretDataObjectHash,omitempty"`

	// LastRetrievedTimestamp represents the timestamp of the last time the secret was retrieved.
	// +optional
	LastRetrievedTimestamp *metav1.Time `json:"lastRetrievedTimestamp,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status

// SecretStoreSync represents the desired state and observed state of the secret synchronization process.
type SecretStoreSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretStoreSyncSpec   `json:"spec,omitempty"`
	Status SecretStoreSyncStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SecretStoreSyncList contains a list of SecretStoreSync resources.
type SecretStoreSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretStoreSync `json:"items"`
}

func init() {
	// SchemeBuilder.Register(&SecretStoreSync{}, &SecretStoreSyncList{})
}
