// copied from https://github.com/kubernetes-sigs/cluster-api/blob/v0.3.6/test/framework/bootstrap/interfaces.go

package bootstrap

import "context"

// ClusterProvider defines the behavior of a type that is responsible for provisioning and managing a Kubernetes cluster.
type ClusterProvider interface {
	// Create a Kubernetes cluster.
	// Generally to be used in the BeforeSuite function to create a Kubernetes cluster to be shared between tests.
	Create(context.Context, string)

	// GetKubeconfigPath returns the path to the kubeconfig file to be used to access the Kubernetes cluster.
	GetKubeconfigPath() string

	// Dispose will completely clean up the provisioned cluster.
	// This should be implemented as a synchronous function.
	// Generally to be used in the AfterSuite function if a Kubernetes cluster is shared between tests.
	// Should try to clean everything up and report any dangling artifacts that needs manual intervention.
	Dispose(context.Context)
}
