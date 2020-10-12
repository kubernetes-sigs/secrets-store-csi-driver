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

package azure

import (
	"context"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
)

const (
	SecretsStoreCreds = "secrets-store-creds"
)

type InstallSecretsStoreCredentialInput struct {
	Creator      framework.Creator
	Namespace    string
	ClientID     string
	ClientSecret string
}

func InstallSecretsStoreCredential(ctx context.Context, input InstallSecretsStoreCredentialInput) {
	e2e.Byf("%s: Installing credential service account", input.Namespace)

	Expect(input.Creator.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretsStoreCreds,
			Namespace: input.Namespace,
		},
		Data: map[string][]byte{
			"clientid":     []byte(input.ClientID),
			"clientsecret": []byte(input.ClientSecret),
		},
	})).To(Succeed())
}
