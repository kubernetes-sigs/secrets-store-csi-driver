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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	authenticationv1 "k8s.io/api/authentication/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	clitesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
)

var (
	testDriver    = "test-driver"
	testAccount   = "test-service-account"
	testPod       = "test-pod"
	testNamespace = "test-ns"
	testUID       = "test-uid"
)

func TestPodServiceAccountTokenAttrs(t *testing.T) {
	scheme := runtime.NewScheme()
	audience := "aud"

	tests := []struct {
		desc                         string
		driver                       *storagev1.CSIDriver
		wantServiceAccountTokenAttrs map[string]string
	}{
		{
			desc: "csi driver has no ServiceAccountToken",
			driver: &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: testDriver,
				},
				Spec: storagev1.CSIDriverSpec{},
			},
			wantServiceAccountTokenAttrs: nil,
		},
		{
			desc: "one token with empty string as audience",
			driver: &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: testDriver,
				},
				Spec: storagev1.CSIDriverSpec{
					TokenRequests: []storagev1.TokenRequest{
						{
							Audience: "",
						},
					},
				},
			},
			wantServiceAccountTokenAttrs: map[string]string{"csi.storage.k8s.io/serviceAccount.tokens": `{"":{"token":"test-ns:test-service-account:3600:[api]","expirationTimestamp":"1970-01-01T00:00:01Z"}}`},
		},
		{
			desc: "one token with non-empty string as audience",
			driver: &storagev1.CSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: testDriver,
				},
				Spec: storagev1.CSIDriverSpec{
					TokenRequests: []storagev1.TokenRequest{
						{
							Audience: audience,
						},
					},
				},
			},
			wantServiceAccountTokenAttrs: map[string]string{"csi.storage.k8s.io/serviceAccount.tokens": `{"aud":{"token":"test-ns:test-service-account:3600:[aud]","expirationTimestamp":"1970-01-01T00:00:01Z"}}`},
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
			client.PrependReactor("create", "serviceaccounts", clitesting.ReactionFunc(func(action clitesting.Action) (bool, runtime.Object, error) {
				tr := action.(clitesting.CreateAction).GetObject().(*authenticationv1.TokenRequest)
				scheme.Default(tr)
				if len(tr.Spec.Audiences) == 0 {
					tr.Spec.Audiences = []string{"api"}
				}
				if tr.Spec.ExpirationSeconds == nil {
					tr.Spec.ExpirationSeconds = pointer.Int64(3600)
				}
				tr.Status.Token = fmt.Sprintf("%v:%v:%d:%v", action.GetNamespace(), testAccount, *tr.Spec.ExpirationSeconds, tr.Spec.Audiences)
				tr.Status.ExpirationTimestamp = metav1.NewTime(time.Unix(1, 1))
				return true, tr, nil
			}))

			tokenClient := NewTokenClient(client, testDriver, 1*time.Second)
			_ = tokenClient.Run(wait.NeverStop)
			waitForInformerCacheSync()

			attrs, _ := tokenClient.PodServiceAccountTokenAttrs(testNamespace, testPod, testAccount, types.UID(testUID))
			if diff := cmp.Diff(test.wantServiceAccountTokenAttrs, attrs); diff != "" {
				t.Errorf("PodServiceAccountTokenAttrs() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}
