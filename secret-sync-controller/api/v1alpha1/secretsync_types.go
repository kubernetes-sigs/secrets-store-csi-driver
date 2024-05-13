/*
Copyright 2024 The Kubernetes Authors.

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
	// sourcePath is the data source value of the secret defined in the Secret Provider Class.
	// This matches the path of a file in the MountResponse returned from the provider.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=^[A-Za-z0-9.]([-A-Za-z0-9]+([-._a-zA-Z0-9]?[A-Za-z0-9])*)?(\/([0-9]+))*$
	// +kubebuilder:validation:Required
	SourcePath string `json:"sourcePath"`

	// targetKey is the key in the Kubernetes secret's data field as described in the Kubernetes API reference:
	// https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/secret-v1/
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=^[A-Za-z0-9.]([-A-Za-z0-9]+([-._a-zA-Z0-9]?[A-Za-z0-9])*)?(\/([0-9]+))*$
	// +kubebuilder:validation:Required
	TargetKey string `json:"targetKey"`
}

// SecretObject defines the desired state of synchronized Kubernetes secret objects.
type SecretObject struct {
	// type specifies the type of the Kubernetes secret object,
	// e.g. "Opaque";"kubernetes.io/basic-auth";"kubernetes.io/ssh-auth";"kubernetes.io/tls"
	// The controller must have permission to create secrets of the specified type.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// data is a list of SecretObjectData containing secret data source from the Secret Provider Class and the
	// corresponding data field key used in the Kubernetes secret object.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	// +listType=map
	// +listMapKey=targetKey
	Data []SecretObjectData `json:"data"`

	// labels contains key-value pairs representing labels associated with the Kubernetes secret object.
	// The labels are used to identify the secret object created by the controller.
	// On secret creation, the following label is added: secrets-store.sync.x-k8s.io/secretsync=<secret-sync-name>.
	// The following label prefix is reserved: secrets-store.sync.x-k8s.io/.
	// Creation fails if the label is specified in the SecretSync object with a different value.
	// On secret update, if the validation admission policy is set, the controller will check if the label
	// secrets-store.sync.x-k8s.io/secretsync=<secret-sync-name> is present. If the label is not present,
	// controller fails to update the secret.
	// +kubebuilder:validation:XValidation:message="Labels should have < 63 characters for both keys and values.",rule="(self.all(x, x.size() < 63 && self[x].size() < 63) == true)"
	// +kubebuilder:validation:XValidation:message="Labels should not contain secrets-store.sync.x-k8s.io. This key is reserved for the controller.",rule="(self.all(x, x.startsWith('secrets-store.sync.x-k8s.io') == false))"
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// annotations contains key-value pairs representing annotations associated with the Kubernetes secret object.
	// The following annotation prefix is reserved: secrets-store.sync.x-k8s.io/.
	// Creation fails if the annotation key is specified in the SecretSync object by the user.
	// +kubebuilder:validation:XValidation:message="Annotations should have < 253 characters for both keys and values.",rule="(self.all(x, x.size() < 253 && self[x].size() < 253) == true)"
	// +kubebuilder:validation:XValidation:message="Annotations should not contain secrets-store.sync.x-k8s.io. This key is reserved for the controller.",rule="(self.all(x, x.startsWith('secrets-store.sync.x-k8s.io') == false))"
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// SecretSyncSpec defines the desired state for synchronizing secret.
type SecretSyncSpec struct {
	// secretSyncControllerName specifies the name of the secret sync controller used to synchronize
	// the secret.
	// +optional
	// +kubebuilder:default:=""
	SecretSyncControllerName string `json:"secretSyncControllerName"`

	// secretProviderClassName specifies the name of the secret provider class used to pass information to
	// access the secret store.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$
	// +kubebuilder:validation:Required
	SecretProviderClassName string `json:"secretProviderClassName"`

	// serviceAccountName specifies the name of the service account used to access the secret store.
	// The audience field in the service account token must be passed as parameter in the controller configuration.
	// The audience is used when requesting a token from the API server for the service account; the supported
	// audiences are defined by each provider.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$
	// +kubebuilder:validation:Required
	ServiceAccountName string `json:"serviceAccountName"`

	// secretObject specifies the configuration for the synchronized Kubernetes secret object.
	// +kubebuilder:validation:Required
	SecretObject SecretObject `json:"secretObject"`

	// forceSynchronization can be used to force the secret synchronization. The secret synchronization is
	// triggered, by changing the value in this field.
	// This field is not used to resolve synchronization conflicts.
	// It is not related with the force query parameter in the Apply operation.
	// https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=^[A-Za-z0-9]([-A-Za-z0-9]+([-._a-zA-Z0-9]?[A-Za-z0-9])*)?
	// +optional
	ForceSynchronization string `json:"forceSynchronization,omitempty"`
}

// SecretSyncStatus defines the observed state of the secret synchronization process.
type SecretSyncStatus struct {
	// syncHash contains the hash of the secret object data, data from the SecretProviderClass (e.g. UID,
	// and metadata.generation), and similar data from the SecretSync. This hash is used to
	// determine if the secret changed.
	// The hash is calculated using the HMAC (Hash-based Message Authentication Code) algorithm, using bcrypt
	// hashing, with the SecretsSync's UID as the key.
	// The secret is updated if:
	//		1. the hash is different
	//		2. the lastSuccessfulSyncTime indicates a rotation is required
	//			- the rotation poll interval is passed as a parameter in the controller configuration
	//		3. the SecretUpdateStatus is 'Failed'
	// +optional
	SyncHash string `json:"syncHash,omitempty"`

	// lastSuccessfulSyncTime represents the last time the secret was retrieved from the Provider and updated.
	// +optional
	LastSuccessfulSyncTime *metav1.Time `json:"lastSuccessfulSyncTime,omitempty"`

	// conditions represent the status of the secret create and update processes.
	// The status is set to True if the secret was created or updated successfully.
	// The status is set to False if the secret create or update failed.
	// The status is set to Unknown if the secret patch failed due to an unknown error.
	// The following conditions are used:
	// 		- Type: Create
	//			- Status: True
	//			  Reason: CreateSucceeded
	//			  Message: The secret was created successfully.
	//			- Status: False
	//			  Reason: ProviderError
	//			  Message: The secret create failed due to a provider error: errorCode, check the logs or the events for more information.
	//			- Status: False
	//			  Reason: InvalidClusterSecretLabelError
	//			  Message: The secret create failed because a label reserved for the controller is applied on the secret.
	//			- Status: False
	//			  Reason: InvalidClusterSecretAnnotationError
	//		      Message: The secret create failed because an annotation reserved for the controller is applied on the secret.
	//			- Status: False
	//			  Reason: UnknownError
	//			  Message: Secret patch failed due to unknown error, check the logs or the events for more information.
	//			- Status: False
	//			  Reason: ValidatingAdmissionPolicyCheckFailed
	//			  Message: The secret update failed because the validating admission policy check failed.
	//			- Status: False
	//			  Reason: UserInputValidationFailed
	//			  Message: The secret update failed because the user input validation failed. (e.g. if a secret type is invalid).
	//			- Status: False
	//			  Reason: ControllerSPCError
	//			  Message: The secret update failed because the controller failed to get the secret provider class, or the SPC is misconfigured.
	//			- Status: False
	//			  Reason: ControllerInternalError
	//			  Message: The secret update failed due to an internal error, check the logs or the events for more information.
	//		- Type: Update
	//			- Status: True
	//			  Reason: NoValueChange
	//			  Message: The secret was updated successfully at the end of the poll interval and no value change was detected.
	//			- Status: True
	//			  Reason: ValueChangeOrForceUpdateDetected
	//			  Message: The secret was updated successfully:a value change or a force update was detected.
	//			- Status: False
	//			  Reason: ValidatingAdmissionPolicyCheckFailed
	//			  Message: The secret update failed because the validating admission policy check failed.
	//			- Status: False
	//			  Reason: InvalidClusterSecretLabelError
	//			  Message: The secret update failed because a label reserved for the controller is applied on the secret.
	//			- Status: False
	//			  Reason: InvalidClusterSecretAnnotationError
	//		      Message: The secret update failed because an annotation reserved for the controller is applied on the secret.
	//			- Status: False
	//			  Reason: ProviderError
	//			  Message: The secret update failed due to a provider error: errorCode, check the logs or the events for more information.
	//			- Status: False
	//			  Reason: UserInputValidationFailed
	//			  Message: The secret update failed because the user input validation failed. (e.g. if a secret type is invalid).
	//			- Status: False
	//			  Reason: ControllerSPCError
	//			  Message: The secret update failed because the controller failed to get the secret provider class, or the SPC is misconfigured.
	//			- Status: False
	//			  Reason: ControllerInternalError
	//			  Message: The secret update failed due to an internal error, check the logs or the events for more information.
	//			- Status: False
	//			  Reason: UnknownError
	//			  Message: Secret patch failed due to unknown error, check the logs or the events for more information.

	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +kubebuilder:validation:MaxItems=16
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +genclient
// +kubebuilder:object:generate:=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// SecretSync represents the desired state and observed state of the secret synchronization process.
// The SecretSync name is used to as the secret object created by the controller.
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
	SchemeBuilder.Register(&SecretSync{}, &SecretSyncList{})
}
