// +build vault

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/csidriver"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/vault"
)

var _ = Describe("Testing CSI Driver with Vault provider", func() {
	ctx := context.TODO()
	csiNamespace := "secrets-store-csi-driver"

	It("Install CSI driver and Vault provider", func() {
		namespace := "secrets-store-csi-driver"
		cli := clusterProxy.GetClient()
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: csiNamespace,
			},
		}
		Eventually(func() error {
			return cli.Create(ctx, ns)
		}, framework.CreateTimeout, framework.CreatePolling).Should(Succeed())

		csidriver.InstallAndWait(context.TODO(), csidriver.InstallAndWaitInput{
			Getter:         cli,
			KubeConfigPath: clusterProxy.GetKubeconfigPath(),
			ChartPath:      chartPath,
			Namespace:      csiNamespace,
		})
		vault.InstallAndWaitProvider(ctx, vault.InstallAndWaitProviderInput{
			Creator:   cli,
			Getter:    cli,
			Namespace: namespace,
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
			csiNamespace: csiNamespace,
			skipCleanup:  skipCleanup,
			chartPath:    chartPath,
			manifestsDir: manifestsDir,
		}
	})
})
