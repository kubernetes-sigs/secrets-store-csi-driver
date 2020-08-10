// copied parcially from https://github.com/kubernetes-sigs/cluster-api/blob/v0.3.6/test/framework/config.go
package framework

// LoadImageBehavior indicates the behavior when loading an image.
type LoadImageBehavior string

const (
	// MustLoadImage causes a load operation to fail if the image cannot be
	// loaded.
	MustLoadImage LoadImageBehavior = "mustLoad"

	// TryLoadImage causes any errors that occur when loading an image to be
	// ignored.
	TryLoadImage LoadImageBehavior = "tryLoad"
)

// ContainerImage describes an image to load into a cluster and the behavior
// when loading the image.
type ContainerImage struct {
	// Name is the fully qualified name of the image.
	Name string

	// LoadBehavior may be used to dictate whether a failed load operation
	// should fail the test run. This is useful when wanting to load images
	// *if* they exist locally, but not wanting to fail if they don't.
	//
	// Defaults to MustLoadImage.
	LoadBehavior LoadImageBehavior
}
