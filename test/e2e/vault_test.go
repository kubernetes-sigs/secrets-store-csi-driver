// +build vault

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

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/vault"
)

var _ = Describe("Testing CSI Driver with Vault provider", func() {
	ctx := context.TODO()

	It("Install Vault provider and Vault", func() {
		cli := clusterProxy.GetClient()
		vault.InstallAndWaitProvider(ctx, vault.InstallAndWaitProviderInput{
			Creator:   cli,
			Getter:    cli,
			Namespace: csiNamespace,
		})
		vault.SetupVault(ctx, vault.SetupVaultInput{
			Creator:        cli,
			GetLister:      cli,
			Namespace:      csiNamespace,
			ManifestsDir:   manifestsDir,
			KubeconfigPath: clusterProxy.GetKubeconfigPath(),
		})
	})

	VaultSpec(ctx, func() VaultSpecInput {
		return VaultSpecInput{
			clusterProxy: clusterProxy,
			skipCleanup:  skipCleanup,
			chartPath:    chartPath,
			manifestsDir: manifestsDir,
		}
	})
})
