package vault

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestE2eVault(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Vault Suite")
}

var _ = BeforeSuite(func() {
	SetDefaultEventuallyPollingInterval(3 * time.Second)
	SetDefaultEventuallyTimeout(9 * time.Minute)
})

var _ = Describe("Test vault provider", func() {
	Context("install vault provider", testInstallVaultProvider)
	Context("CSI inline volume", testCSIInlineVolume)
	Context("Sync with K8s secrets", testSyncWithK8sSecrets)
})
