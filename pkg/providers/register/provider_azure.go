// +build !no_azure_provider

package register

import (
	"github.com/deislabs/secrets-store-csi-driver/pkg/providers"
	"github.com/deislabs/secrets-store-csi-driver/pkg/providers/azure"
)

func init() {
	register("azure", initAzure)
}

func initAzure(cfg InitConfig) (providers.Provider, error) {
	return azure.NewProvider()
}
