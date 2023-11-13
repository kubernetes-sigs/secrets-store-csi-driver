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

package k8sutil

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

func TestSPCVolume(t *testing.T) {
	tests := []struct {
		name    string
		pod     *corev1.Pod
		spcName string
		want    *corev1.Volume
	}{
		{
			name:    "No Volume",
			pod:     &corev1.Pod{},
			spcName: "foo",
			want:    nil,
		},
		{
			name: "No CSI Volume",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name:         "non-csi-volume",
							VolumeSource: corev1.VolumeSource{},
						},
					},
				},
			},
			spcName: "foo",
			want:    nil,
		},
		{
			name: "CSI volume but wrong driver",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "csi-volume",
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									Driver: "example-driver.k8s.io",
								},
							},
						},
					},
				},
			},
			spcName: "foo",
			want:    nil,
		},
		{
			name: "Wrong Volume",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "csi-volume",
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									Driver:           "secrets-store.csi.k8s.io",
									VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
								},
							},
						},
					},
				},
			},
			spcName: "foo",
			want:    nil,
		},
		{
			name: "Correct Volume",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "csi-volume",
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									Driver:           "secrets-store.csi.k8s.io",
									VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
								},
							},
						},
					},
				},
			},
			spcName: "spc1",
			want: &corev1.Volume{
				Name: "csi-volume",
				VolumeSource: corev1.VolumeSource{
					CSI: &corev1.CSIVolumeSource{
						Driver:           "secrets-store.csi.k8s.io",
						VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
					},
				},
			},
		},
		{
			name: "Multiple Volumes",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "csi-volume-0",
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									Driver:           "secrets-store.csi.k8s.io",
									VolumeAttributes: map[string]string{"secretProviderClass": "spc0"},
								},
							},
						},
						{
							Name: "csi-volume",
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									Driver:           "secrets-store.csi.k8s.io",
									VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
								},
							},
						},
					},
				},
			},
			spcName: "spc1",
			want: &corev1.Volume{
				Name: "csi-volume",
				VolumeSource: corev1.VolumeSource{
					CSI: &corev1.CSIVolumeSource{
						Driver:           "secrets-store.csi.k8s.io",
						VolumeAttributes: map[string]string{"secretProviderClass": "spc1"},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SPCVolume(tc.pod, "secrets-store.csi.k8s.io", tc.spcName)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("SPCVolume() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
