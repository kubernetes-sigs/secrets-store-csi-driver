// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Provider enum for all the provider names
type Provider string

const (
	// Azure provider for Azure Key Vault
	Azure Provider = "Azure"
	// Vault provider for Hashicorp Vault
	Vault Provider = "Vault"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SecretObjectData defines the desired state of synced K8s secret object data
type SecretObjectData struct {
	// name of the object to sync
	ObjectName string `json:"objectName,omitempty"`
	// data field to populate
	Key string `json:"key,omitempty"`
}

// SecretObject defines the desired state of synced K8s secret objects
type SecretObject struct {
	// name of the K8s secret object
	SecretName string `json:"secretName,omitempty"`
	// type of K8s secret object
	Type string              `json:"type,omitempty"`
	Data []*SecretObjectData `json:"data,omitempty"`
}

// SecretProviderClassSpec defines the desired state of SecretProviderClass
type SecretProviderClassSpec struct {
	// Configuration for provider name
	Provider Provider `json:"provider,omitempty"`
	// Configuration for specific provider
	Parameters    map[string]string `json:"parameters,omitempty"`
	SecretObjects []*SecretObject   `json:"secretObjects,omitempty"`
}

// ByPodStatus defines the state of SecretProviderClass as seen by
// an individual controller
type ByPodStatus struct {
	// id of the pod that wrote the status
	ID string `json:"id,omitempty"`
	// namespace of the pod that wrote the status
	Namespace string `json:"namespace,omitempty"`
}

// SecretRefStatus defines the state of secret objects
type SecretRefStatus struct {
	Name    string `json:"name,omitempty"`
	Created bool   `json:"created,omitempty"`
}

// SecretProviderClassStatus defines the observed state of SecretProviderClass
type SecretProviderClassStatus struct {
	ByPod     []*ByPodStatus     `json:"byPod,omitempty"`
	SecretRef []*SecretRefStatus `json:"secretRef,omitempty"`
}

// +kubebuilder:object:root=true

// SecretProviderClass is the Schema for the secretproviderclasses API
type SecretProviderClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretProviderClassSpec   `json:"spec,omitempty"`
	Status SecretProviderClassStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecretProviderClassList contains a list of SecretProviderClass
type SecretProviderClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretProviderClass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecretProviderClass{}, &SecretProviderClassList{})
}
