// +build !no_azure_provider

package register

import (
	"github.com/ritazh/keyvault-csi-driver/pkg/providers"
	"github.com/ritazh/keyvault-csi-driver/pkg/providers/azure"
)

func init() {
	register("azure", initAzure)
}

func initAzure(cfg InitConfig) (providers.Provider, error) {
	return azure.NewProvider()
}
