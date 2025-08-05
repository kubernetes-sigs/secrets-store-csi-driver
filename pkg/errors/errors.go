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

package errors

const (
	// FailedToEnsureMountPoint error
	FailedToEnsureMountPoint = "FailedToEnsureMountPoint"
	// IncompatibleProviderVersion error
	IncompatibleProviderVersion = "IncompatibleProviderVersion"
	// ProviderError error
	ProviderError = "ProviderError"
	// FailedToMount error
	FailedToMount = "FailedToMount"
	// SecretProviderClassNotFound error
	SecretProviderClassNotFound = "SecretProviderClassNotFound"
	// FailedToLookupProviderGRPCClient error
	FailedToLookupProviderGRPCClient = "FailedToLookupProviderGRPCClient"
	// GRPCProviderError error
	GRPCProviderError = "GRPCProviderError"
	// FailedToRotate error
	FailedToRotate = "FailedToRotate"
	// PodNotFound error
	PodNotFound = "PodNotFound"
	// NodePublishSecretRefNotFound error
	// #nosec G101 (Ref: https://github.com/securego/gosec/issues/295)
	NodePublishSecretRefNotFound = "NodePublishSecretRefNotFound"
	// UnexpectedTargetPath error
	// Indicated SecretProviderClassPodStatus status.targetPath is an invalid value.
	UnexpectedTargetPath = "UnexpectedTargetPath"
	// PodVolumeNotFound error
	PodVolumeNotFound = "PodVolumeNotFound"
	// FileWriteError error
	FileWriteError       = "FileWriteError"
	FailedToParseFSGroup = "FailedToParseFSGroup"
)
