// +build azure

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Testing CSI Driver with Azure provider", func() {
	AzureSpec(context.TODO(), func() AzureSpecInput {
		return VaultSpecInput{
			clusterProxy: clusterProxy,
			SkipCleanup:  skipCleanup,
			chartPath:    chartPath,
			manifestsDir: manifestsDir,
		}
	})
})
