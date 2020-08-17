/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// copied from https://github.com/kubernetes-sigs/cluster-api/blob/v0.3.6/test/framework/bootstrap/kind_util.go
// and modified

package bootstrap

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"k8s.io/klog"
	kind "sigs.k8s.io/kind/pkg/cluster"
	kindnodes "sigs.k8s.io/kind/pkg/cluster/nodes"
	kindnodesutils "sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework"
)

// CreateKindClusterAndLoadImagesInput is the input for CreateKindClusterAndLoadImages.
type CreateKindClusterAndLoadImagesInput struct {
	// Name of the cluster
	Name string

	// RequiresDockerSock defines if the cluster requires the docker sock
	RequiresDockerSock bool

	// Images to be loaded in the cluster (this is kind specific)
	Images []framework.ContainerImage

	// KindconfigPath is kind configuration file path
	KindconfigPath string

	// NodeImage is kindest/node image tag
	NodeImage string
}

// CreateKindClusterAndLoadImages returns a new Kubernetes cluster with pre-loaded images.
func CreateKindClusterAndLoadImages(ctx context.Context, input CreateKindClusterAndLoadImagesInput) ClusterProvider {
	Expect(ctx).NotTo(BeNil(), "ctx is required for CreateKindClusterAndLoadImages")
	Expect(input.Name).ToNot(BeEmpty(), "Invalid argument. Name can't be empty when calling CreateKindClusterAndLoadImages")

	klog.Infof("Creating a kind cluster with name %q", input.Name)

	options := []KindClusterOption{}
	if input.KindconfigPath != "" {
		options = append(options, WithKindConfig(input.KindconfigPath))
	}
	if input.RequiresDockerSock {
		options = append(options, WithDockerSockMount())
	}
	clusterProvider := NewKindClusterProvider(input.Name, options...)
	Expect(clusterProvider).ToNot(BeNil(), "Failed to create a kind cluster")

	clusterProvider.Create(ctx, input.NodeImage)
	Expect(clusterProvider.GetKubeconfigPath()).To(BeAnExistingFile(), "The kubeconfig file for the kind cluster with name %q does not exists at %q as expected", input.Name, clusterProvider.GetKubeconfigPath())

	LoadImagesToKindCluster(ctx, LoadImagesToKindClusterInput{
		Name:   input.Name,
		Images: input.Images,
	})

	return clusterProvider
}

// LoadImagesToKindClusterInput is the input for LoadImagesToKindCluster.
type LoadImagesToKindClusterInput struct {
	// Name of the cluster
	Name string

	// Images to be loaded in the cluster (this is kind specific)
	Images []framework.ContainerImage
}

// LoadImagesToKindCluster provides a utility for loading images into a kind cluster.
func LoadImagesToKindCluster(ctx context.Context, input LoadImagesToKindClusterInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for LoadImagesToKindCluster")
	Expect(input.Name).ToNot(BeEmpty(), "Invalid argument. Name can't be empty when calling LoadImagesToKindCluster")

	for _, image := range input.Images {
		klog.Infof("Loading image: %q", image.Name)
		err := loadImage(ctx, input.Name, image.Name)
		switch image.LoadBehavior {
		case framework.MustLoadImage:
			Expect(err).ToNot(HaveOccurred(), "Failed to load image %q into the kind cluster %q", image.Name, input.Name)
		case framework.TryLoadImage:
			if err != nil {
				klog.Warningf("[WARNING] Unable to load image %q into the kind cluster %q: %v", image.Name, input.Name, err)
			}
		}
	}
}

// LoadImage will put a local image onto the kind node
func loadImage(ctx context.Context, cluster, image string) error {
	// Save the image into a tar
	dir, err := ioutil.TempDir("", "image-tar")
	if err != nil {
		return errors.Wrap(err, "failed to create tempdir")
	}
	defer os.RemoveAll(dir)
	imageTarPath := filepath.Join(dir, "image.tar")

	err = save(image, imageTarPath)
	if err != nil {
		return err
	}

	// Gets the nodes in the cluster
	provider := kind.NewProvider()
	nodeList, err := provider.ListInternalNodes(cluster)
	if err != nil {
		return err
	}

	// Load the image on the selected nodes
	for _, node := range nodeList {
		if err := load(imageTarPath, node); err != nil {
			return err
		}
	}

	return nil
}

// copied from kind https://github.com/kubernetes-sigs/kind/blob/v0.7.0/pkg/cmd/kind/load/docker-image/docker-image.go#L168
func save(image, dest string) error {
	return exec.Command("docker", "save", "-o", dest, image).Run()
}

// copied from kind https://github.com/kubernetes-sigs/kind/blob/v0.7.0/pkg/cmd/kind/load/docker-image/docker-image.go#L158
// loads an image tarball onto a node
func load(imageTarName string, node kindnodes.Node) error {
	f, err := os.Open(imageTarName)
	if err != nil {
		return errors.Wrap(err, "failed to open image")
	}
	defer f.Close()
	return kindnodesutils.LoadImageArchive(node, f)
}
