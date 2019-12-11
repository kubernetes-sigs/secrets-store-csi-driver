/*
Copyright 2019 The Kubernetes Authors.
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

package version

import (
	"reflect"
	"testing"
)

func TestGetMinimumProviderVersions(t *testing.T) {
	cases := []struct {
		desc                string
		minProviderVersions string
		expectedMap         map[string]string
		expectedErr         bool
	}{
		{
			desc:                "no min provider version provided",
			minProviderVersions: "",
			expectedMap:         make(map[string]string),
			expectedErr:         false,
		},
		{
			desc:                "using ; instead of , as delimiter",
			minProviderVersions: "provider1=0.0.2;provider2=0.0.4",
			expectedMap:         make(map[string]string),
			expectedErr:         true,
		},
		{
			desc:                "invalid provider version",
			minProviderVersions: " = ",
			expectedMap:         make(map[string]string),
			expectedErr:         true,
		},
		{
			desc:                "invalid provider version(2)",
			minProviderVersions: "provider1=0.0.2,provider2= ",
			expectedMap:         map[string]string{"provider1": "0.0.2"},
			expectedErr:         true,
		},
		{
			desc:                "provider version bad format",
			minProviderVersions: "provider1:0.0.2,provider2=0.0.4",
			expectedMap:         make(map[string]string),
			expectedErr:         true,
		},
		{
			desc:                "duplicate provider version",
			minProviderVersions: "provider1=0.0.2,provider1=0.0.4",
			expectedMap:         map[string]string{"provider1": "0.0.2"},
			expectedErr:         true,
		},
		{
			desc:                "invalid semver",
			minProviderVersions: "provider1=v0.0.2",
			expectedMap:         make(map[string]string),
			expectedErr:         true,
		},
		{
			desc:                "single min provider version provided",
			minProviderVersions: "provider1=0.0.2",
			expectedMap:         map[string]string{"provider1": "0.0.2"},
			expectedErr:         false,
		},
		{
			desc:                "more than one provider version provided",
			minProviderVersions: "provider1=0.0.2,provider2=0.0.4",
			expectedMap:         map[string]string{"provider1": "0.0.2", "provider2": "0.0.4"},
			expectedErr:         false,
		},
		{
			desc:                "white space in min provider versions",
			minProviderVersions: "provider1=0.0.2, provider2=0.0.4",
			expectedMap:         map[string]string{"provider1": "0.0.2", "provider2": "0.0.4"},
			expectedErr:         false,
		},
		{
			desc:                "more white space in min provider versions",
			minProviderVersions: "provider1=0.0.2 , provider2=0.0.4",
			expectedMap:         map[string]string{"provider1": "0.0.2", "provider2": "0.0.4"},
			expectedErr:         false,
		},
		{
			desc:                "more white space in min provider versions(2)",
			minProviderVersions: "provider1 = 0.0.2 , provider2  =  0.0.4",
			expectedMap:         map[string]string{"provider1": "0.0.2", "provider2": "0.0.4"},
			expectedErr:         false,
		},
	}

	for i, tc := range cases {
		t.Log(i, tc.desc)
		actualMap, err := GetMinimumProviderVersions(tc.minProviderVersions)
		if (err != nil) != tc.expectedErr {
			t.Fatalf("expected error: %v, actual: %v", tc.expectedErr, err)
		}
		if !reflect.DeepEqual(tc.expectedMap, actualMap) {
			t.Fatalf("expected: %v, actual: %v", tc.expectedMap, actualMap)
		}
	}
}

func TestIsProviderCompatible(t *testing.T) {
	cases := []struct {
		desc        string
		currVersion string
		minVersion  string
		expected    bool
		expectedErr bool
	}{
		{
			desc:        "empty version",
			currVersion: "",
			minVersion:  "0.0.4",
			expected:    false,
			expectedErr: true,
		},
		{
			desc:        "invalid version semver",
			currVersion: "v0.0.2",
			minVersion:  "v0.0.4",
			expected:    false,
			expectedErr: true,
		},
		{
			desc:        "curr version < min version",
			currVersion: "0.0.2",
			minVersion:  "0.0.4",
			expected:    false,
			expectedErr: false,
		},
		{
			desc:        "curr version > min version",
			currVersion: "0.0.6",
			minVersion:  "0.0.4",
			expected:    true,
			expectedErr: false,
		},
		{
			desc:        "curr version = min version",
			currVersion: "0.0.4",
			minVersion:  "0.0.4",
			expected:    true,
			expectedErr: false,
		},
	}

	for i, tc := range cases {
		t.Log(i, tc.desc)
		actual, err := isProviderCompatible(tc.currVersion, tc.minVersion)
		if err == nil && tc.expectedErr {
			t.Fatalf("expecting err: %v, got %v", tc.expectedErr, err)
		}
		if tc.expected != actual {
			t.Fatalf("expected: %v, actual: %v", tc.expected, actual)
		}
	}
}
