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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kubernetes-csi/csi-test/v4/pkg/sanity"

	secretsstore "sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/version"
)

const (
	socket             = "/tmp/csi.sock"
	endpoint           = "unix://" + socket
	providerVolumePath = "/etc/kubernetes/secrets-store-csi-providers"
)

func TestSanity(t *testing.T) {
	driver := secretsstore.GetDriver()
	go func() {
		driver.Run(context.Background(), "secrets-store.csi.k8s.io", "somenodeid", endpoint, providerVolumePath, "provider1=0.0.2,provider2=0.0.4", nil, nil)
	}()

	tmpPath := filepath.Join(os.TempDir(), "csi")
	config := sanity.NewTestConfig()
	config.Address = endpoint
	config.CreateTargetDir = func(targetPath string) (string, error) {
		targetPath = filepath.Join(tmpPath, targetPath)
		return targetPath, createTargetDir(targetPath)
	}
	config.RemoveTargetPath = func(targetPath string) error {
		return os.RemoveAll(targetPath)
	}

	version.BuildVersion = "mock"
	version.BuildTime = time.Now().String()
	sanity.Test(t, config)
}

func createTargetDir(targetPath string) error {
	fileInfo, err := os.Stat(targetPath)
	if err != nil && os.IsNotExist(err) {
		return os.MkdirAll(targetPath, 0755)
	} else if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		return fmt.Errorf("target location %s is not a directory", targetPath)
	}

	return nil
}
