// +build vault

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Testing CSI Driver with Vault provider", func() {

	VaultSpec(context.TODO(), func() VaultSpecInput {
		return VaultSpecInput{
			clusterProxy: clusterProxy,
			SkipCleanup:  skipCleanup,
			chartPath:    chartPath,
			manifestsDir: manifestsDir,
		}
	})

})
