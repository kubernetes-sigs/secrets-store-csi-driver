// +build !no_mock_provider

package register

import (
	"github.com/deislabs/secrets-store-csi-driver/pkg/providers"
	"github.com/deislabs/secrets-store-csi-driver/pkg/providers/mock"
)

func init() {
	register("mock_provider", initMockProvider)
}

func initMockProvider(cfg InitConfig) (providers.Provider, error) {
	return mock.NewProvider()
}
