// +build !no_vault_provider

package register

import (
	"github.com/ritazh/keyvault-csi-driver/pkg/providers"
	"github.com/ritazh/keyvault-csi-driver/pkg/providers/vault"
)

func init() {
	register("vault", initVault)
}

func initVault(cfg InitConfig) (providers.Provider, error) {
	return vault.NewProvider()
}
