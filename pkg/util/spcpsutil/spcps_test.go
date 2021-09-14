/*
Copyright 2021 The Kubernetes Authors.

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

package spcpsutil

import (
	"reflect"
	"testing"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
)

func TestOrderSecretProviderClassObjectByID(t *testing.T) {
	tests := []struct {
		name string
		objs []v1alpha1.SecretProviderClassObject
		want []v1alpha1.SecretProviderClassObject
	}{
		{
			name: "empty",
			objs: []v1alpha1.SecretProviderClassObject{},
			want: []v1alpha1.SecretProviderClassObject{},
		},
		{
			name: "one object",
			objs: []v1alpha1.SecretProviderClassObject{
				{
					ID:      "a",
					Version: "v1",
				},
			},
			want: []v1alpha1.SecretProviderClassObject{
				{
					ID:      "a",
					Version: "v1",
				},
			},
		},
		{
			name: "two objects",
			objs: []v1alpha1.SecretProviderClassObject{
				{
					ID:      "a",
					Version: "v1",
				},
				{
					ID:      "b",
					Version: "v2",
				},
			},
			want: []v1alpha1.SecretProviderClassObject{
				{
					ID:      "a",
					Version: "v1",
				},
				{
					ID:      "b",
					Version: "v2",
				},
			},
		},
		{
			name: "unsorted",
			objs: []v1alpha1.SecretProviderClassObject{
				{
					ID:      "c",
					Version: "v1",
				},
				{
					ID:      "a",
					Version: "v2",
				},
				{
					ID:      "b",
					Version: "v3",
				},
			},
			want: []v1alpha1.SecretProviderClassObject{
				{
					ID:      "a",
					Version: "v2",
				},
				{
					ID:      "b",
					Version: "v3",
				},
				{
					ID:      "c",
					Version: "v1",
				},
			},
		},
		{
			name: "nested ids",
			objs: []v1alpha1.SecretProviderClassObject{
				{
					ID:      "secret/secret1",
					Version: "v1",
				},
				{
					ID:      "secret/secret3",
					Version: "v2",
				},
				{
					ID:      "secret/secret2",
					Version: "v3",
				},
			},
			want: []v1alpha1.SecretProviderClassObject{
				{
					ID:      "secret/secret1",
					Version: "v1",
				},
				{
					ID:      "secret/secret2",
					Version: "v3",
				},
				{
					ID:      "secret/secret3",
					Version: "v2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OrderSecretProviderClassObjectByID(tt.objs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OrderSecretProviderClassObjectByID() = %v, want %v", got, tt.want)
			}
		})
	}
}
