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

package sanity

import (
	"testing"

	"github.com/kubernetes-csi/csi-test/pkg/sanity"

	secretsstore "github.com/deislabs/secrets-store-csi-driver/pkg/secrets-store"
)

const (
	mountPath          = "/tmp/csi/mount"
	stagePath          = "/tmp/csi/stage"
	socket             = "/tmp/csi.sock"
	endpoint           = "unix://" + socket
	providerVolumePath = "/etc/kubernetes/secrets-store-csi-providers"
)

func TestSanity(t *testing.T) {
	driver := secretsstore.GetDriver()
	go func() {
		driver.Run("secrets-store.csi.k8s.com", "somenodeid", endpoint, providerVolumePath, "")
	}()

	config := &sanity.Config{
		TargetPath:  mountPath,
		StagingPath: stagePath,
		Address:     endpoint,
	}
	sanity.Test(t, config)
}
