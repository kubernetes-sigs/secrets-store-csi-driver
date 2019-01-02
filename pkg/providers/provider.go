package providers

import (
	"os"
	"golang.org/x/net/context"
)

// Provider contains the methods required to implement a keyvault csi provider.
type Provider interface {
	// MountKeyVaultObjectContent mounts content of the keyvault object to target path
	MountKeyVaultObjectContent(ctx context.Context, attrib map[string]string, secrets map[string]string, targetPath string, permission os.FileMode) error
}