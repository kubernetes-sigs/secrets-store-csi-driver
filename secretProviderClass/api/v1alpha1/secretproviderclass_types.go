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

// SecretProviderClassSpec defines the desired state of SecretProviderClass
type SecretProviderClassSpec struct {
	// Configuration for provider name
	Provider Provider `json:"provider,omitempty"`
	// Configuration for specific provider
	Parameters map[string]string `json:"parameters,omitempty"`
}

// SecretProviderClassStatus defines the observed state of SecretProviderClass
type SecretProviderClassStatus struct {
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
