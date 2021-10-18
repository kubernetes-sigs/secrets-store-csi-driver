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

// Package k8sutil holds Secrets CSI Driver utilities for dealing with k8s
// types.
package k8sutil

import (
	corev1 "k8s.io/api/core/v1"
)

// SPCVolume finds the Secret Provider Class volume from a Pod, or returns nil
// if a volume could not be found.
func SPCVolume(pod *corev1.Pod, spcName string) *corev1.Volume {
	for i, vol := range pod.Spec.Volumes {
		if vol.CSI == nil {
			continue
		}
		if vol.CSI.Driver != "secrets-store.csi.k8s.io" {
			continue
		}
		if vol.CSI.VolumeAttributes["secretProviderClass"] != spcName {
			continue
		}
		return &pod.Spec.Volumes[i]
	}
	return nil
}
