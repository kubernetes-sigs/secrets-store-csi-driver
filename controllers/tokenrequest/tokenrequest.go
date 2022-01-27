/*
Copyright 2022 The Kubernetes Authors.

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

// Package tokenrequest holds the RBAC permission annotations for the controller
// to create a serviceaccount token and pass it as part of Mount Request.
// ref: https://kubernetes-csi.github.io/docs/token-requests.html
package tokenrequest

// +kubebuilder:rbac:groups="storage.k8s.io",resources=csidrivers,verbs=get;list;watch,resourceNames=secrets-store.csi.k8s.io
// +kubebuilder:rbac:groups="",resources="serviceaccounts/token",verbs=create
