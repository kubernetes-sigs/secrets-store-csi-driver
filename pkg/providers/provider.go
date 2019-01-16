package providers

import (
	"os"

	"golang.org/x/net/context"
)

// Provider contains the methods required to implement a SecretsStore csi provider.
type Provider interface {
	// MountSecretsStoreObjectContent mounts content of the secrets store object to target path
	MountSecretsStoreObjectContent(ctx context.Context, attrib map[string]string, secrets map[string]string, targetPath string, permission os.FileMode) error
}
