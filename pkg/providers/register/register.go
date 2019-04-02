package register

import (
	"github.com/deislabs/secrets-store-csi-driver/pkg/providers"
	"github.com/pkg/errors"
)

var providerInits = make(map[string]initFunc)

// InitConfig is the config passed to initialize a registered provider.
type InitConfig struct {
	Name string
}

type initFunc func(InitConfig) (providers.Provider, error)

// GetProvider gets the provider specified by the given name
func GetProvider(name string, cfg InitConfig) (providers.Provider, error) {

	f, ok := providerInits[name]
	if !ok {
		return nil, errors.Errorf("provider not found: %s", name)
	}

	return f(cfg)
}

func register(name string, f initFunc) {
	providerInits[name] = f
}
