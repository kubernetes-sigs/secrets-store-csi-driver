// copied from https://github.com/kubernetes-sigs/cluster-api/blob/v0.3.6/test/framework/bootstrap/kind_provider.go
// and modified

package bootstrap

import (
	"context"
	"io/ioutil"
	"os"

	. "github.com/onsi/gomega"

	"k8s.io/klog"
	kindv1 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
	kind "sigs.k8s.io/kind/pkg/cluster"
)

// KindClusterOption is a NewKindClusterProvider option
type KindClusterOption interface {
	apply(*kindClusterProvider)
}

type kindClusterOptionAdapter func(*kindClusterProvider)

func (adapter kindClusterOptionAdapter) apply(kindClusterProvider *kindClusterProvider) {
	adapter(kindClusterProvider)
}

// WithDockerSockMount implements a New Option that instruct the kindClusterProvider to mount /var/run/docker.sock into
// the new kind cluster.
func WithDockerSockMount() KindClusterOption {
	return kindClusterOptionAdapter(func(k *kindClusterProvider) {
		k.withDockerSock = true
	})
}

// WithKindConfig implements a New Option that load kind configuration file for the new kind cluster.
func WithKindConfig(kindconfigPath string) KindClusterOption {
	return kindClusterOptionAdapter(func(k *kindClusterProvider) {
		k.kindconfigPath = kindconfigPath
	})
}

// NewKindClusterProvider returns a ClusterProvider that can create a kind cluster.
func NewKindClusterProvider(name string, options ...KindClusterOption) *kindClusterProvider {
	Expect(name).ToNot(BeEmpty(), "name is required for NewKindClusterProvider")

	clusterProvider := &kindClusterProvider{
		name: name,
	}
	for _, option := range options {
		option.apply(clusterProvider)
	}
	return clusterProvider
}

// kindClusterProvider implements a ClusterProvider that can create a kind cluster.
type kindClusterProvider struct {
	name           string
	withDockerSock bool
	kubeconfigPath string
	kindconfigPath string
}

// Create a Kubernetes cluster using kind.
func (k *kindClusterProvider) Create(ctx context.Context, nodeImage string) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for Create")

	// Sets the kubeconfig path to a temp file.
	// NB. the ClusterProvider is responsible for the cleanup of this file
	f, err := ioutil.TempFile("", "e2e-kind")
	Expect(err).ToNot(HaveOccurred(), "Failed to create kubeconfig file for the kind cluster %q", k.name)
	k.kubeconfigPath = f.Name()

	// Creates the kind cluster
	k.createKindCluster(nodeImage)
}

// createKindCluster calls the kind library taking care of passing options for:
// - use a dedicated kubeconfig file (test should not alter the user environment)
// - if required, mount /var/run/docker.sock
func (k *kindClusterProvider) createKindCluster(nodeImage string) {
	kindCreateOptions := []kind.CreateOption{
		kind.CreateWithKubeconfigPath(k.kubeconfigPath),
	}
	if k.withDockerSock {
		kindCreateOptions = append(kindCreateOptions, kind.CreateWithV1Alpha4Config(withDockerSockConfig()))
	}

	if k.kindconfigPath != "" {
		kindCreateOptions = append(kindCreateOptions, cluster.CreateWithConfigFile(k.kindconfigPath))
	}

	if nodeImage != "" {
		kindCreateOptions = append(kindCreateOptions, cluster.CreateWithNodeImage(nodeImage))
	}

	err := kind.NewProvider().Create(k.name, kindCreateOptions...)
	Expect(err).ToNot(HaveOccurred(), "Failed to create the kind cluster %q")
}

// withDockerSockConfig returns a kind config for mounting /var/run/docker.sock into the kind node.
func withDockerSockConfig() *kindv1.Cluster {
	cfg := &kindv1.Cluster{
		TypeMeta: kindv1.TypeMeta{
			APIVersion: "kind.x-k8s.io/v1alpha4",
			Kind:       "Cluster",
		},
	}
	kindv1.SetDefaultsCluster(cfg)
	cfg.Nodes = []kindv1.Node{
		{
			Role: kindv1.ControlPlaneRole,
			ExtraMounts: []kindv1.Mount{
				{
					HostPath:      "/var/run/docker.sock",
					ContainerPath: "/var/run/docker.sock",
				},
			},
		},
	}
	return cfg
}

// GetKubeconfigPath returns the path to the kubeconfig file for the cluster.
func (k *kindClusterProvider) GetKubeconfigPath() string {
	return k.kubeconfigPath
}

// Dispose the kind cluster and its kubeconfig file.
func (k *kindClusterProvider) Dispose(ctx context.Context) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for Dispose")

	if err := kind.NewProvider().Delete(k.name, k.kubeconfigPath); err != nil {
		klog.Errorf("Deleting the kind cluster %q failed. You may need to remove this by hand.", k.name)
	}
	if err := os.Remove(k.kubeconfigPath); err != nil {
		klog.Errorf("Deleting the kubeconfig file %q file. You may need to remove this by hand.", k.kubeconfigPath)
	}
}
