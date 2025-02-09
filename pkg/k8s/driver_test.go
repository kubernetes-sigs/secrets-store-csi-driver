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

package k8s

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"
)

var driverName = "secrets-store.csi.k8s.io"

func TestGetDriver(t *testing.T) {
	scheme := runtime.NewScheme()

	tests := []struct {
		desc    string
		driver  *storagev1.CSIDriver
		wantErr error
	}{
		{
			desc: "csi driver exists",
			driver: &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: driverName,
				},
				Spec: storagev1.CSIDriverSpec{
					RequiresRepublish: pointer.Bool(true),
				},
			},
			wantErr: nil,
		},
		{
			desc:   "csi driver does not exist",
			driver: nil,
			wantErr: &errors.StatusError{
				ErrStatus: metav1.Status{
					Status:  "Failure",
					Message: `csidriver.storage.k8s.io "secrets-store.csi.k8s.io" not found`,
					Reason:  "NotFound",
					Details: &metav1.StatusDetails{Name: "secrets-store.csi.k8s.io", Group: "storage.k8s.io", Kind: "csidriver"},
					Code:    404,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			client := fakeclient.NewSimpleClientset()
			if test.driver != nil {
				test.driver.Spec.VolumeLifecycleModes = []storagev1.VolumeLifecycleMode{
					storagev1.VolumeLifecycleEphemeral,
				}
				scheme.Default(test.driver)
				client = fakeclient.NewSimpleClientset(test.driver)
			}

			driverClient := NewDriverClient(client, driverName, 1*time.Second)
			_ = driverClient.Run(wait.NeverStop)
			waitForInformerCacheSync()

			_, err := driverClient.GetDriver()
			if diff := cmp.Diff(test.wantErr, err); diff != "" {
				t.Errorf("GetDriver() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}
