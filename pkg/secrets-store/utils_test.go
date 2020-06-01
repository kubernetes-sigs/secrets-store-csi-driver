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

package secretsstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetProviderPath(t *testing.T) {
	cases := []struct {
		providerVolumePath     string
		providerName           string
		goos                   string
		expectedProviderBinary string
	}{
		{
			providerVolumePath:     "/etc/kubernetes/secrets-store-csi-providers",
			providerName:           "p1",
			expectedProviderBinary: "/etc/kubernetes/secrets-store-csi-providers/p1/provider-p1",
		},
		{
			providerVolumePath:     "C:\\k\\secrets-store-csi-providers",
			providerName:           "p1",
			goos:                   "windows",
			expectedProviderBinary: "C:\\k\\secrets-store-csi-providers\\p1\\provider-p1.exe",
		},
	}

	for _, tc := range cases {
		testNodeServer, err := newNodeServer(NewFakeDriver(), tc.providerVolumePath, "", "test-node")
		assert.NoError(t, err)
		assert.NotNil(t, testNodeServer)

		actualProviderBinary := testNodeServer.getProviderPath(tc.goos, tc.providerName)
		assert.Equal(t, tc.expectedProviderBinary, actualProviderBinary)
	}
}
