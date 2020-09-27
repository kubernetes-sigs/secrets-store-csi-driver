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

package e2e

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/bootstrap"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/csidriver"
)

// Test suite flags
var (
	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool

	// nodeImage is kindest/node image tag.
	nodeImage string

	// chartPath is helm chart path.
	chartPath string

	// manifestsDir is manifests directory path.
	manifestsDir string

	// kindconfigPath is kind configuration file path.
	kindconfigPath string

	// tryLoadImages is a list of image tag if exists in local. loading from local will test faster.
	tryLoadImages string

	// MustLoadImages is a list of image tag that must be in local.
	mustLoadImages string
)

// Test suite global vars
var (
	// clusterProvider manages provisioning of the the bootstrap cluster to be used for the e2e tests.
	// Please note that provisioning will be skipped if e2e.use-existing-cluster is provided.
	clusterProvider bootstrap.ClusterProvider

	// clusterProxy allows to interact with the kind cluster to be used for the e2e tests.
	clusterProxy framework.ClusterProxy
)

const csiNamespace = "secrets-store-csi-driver"

func init() {
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.StringVar(&nodeImage, "e2e.node-image", "", "kindest/node image tag")
	flag.StringVar(&chartPath, "e2e.chart-path", "", "helm chart path")
	flag.StringVar(&manifestsDir, "e2e.manifests-dir", "", "manifests directory path")
	flag.StringVar(&kindconfigPath, "e2e.kindconfig-path", "", "kind configuration file path")
	flag.StringVar(&tryLoadImages, "e2e.try-load-images", "", "space separated list of image tag if exists in local")
	flag.StringVar(&mustLoadImages, "e2e.must-load-images", "", "space separated list of image tag that must be in local")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "secrets-store-csi-driver")
}

var _ = BeforeSuite(func() {
	By("Setting up the cluster")
	clusterProvider, clusterProxy = setupCluster(initScheme())

	ctx := context.TODO()

	By("Install CSI driver")
	cli := clusterProxy.GetClient()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: csiNamespace,
		},
	}
	Eventually(func() error {
		return cli.Create(ctx, ns)
	}, framework.CreateTimeout, framework.CreatePolling).Should(Succeed())

	csidriver.InstallAndWait(ctx, csidriver.InstallAndWaitInput{
		Getter:         cli,
		KubeConfigPath: clusterProxy.GetKubeconfigPath(),
		ChartPath:      chartPath,
		Namespace:      csiNamespace,
	})
})

var _ = AfterSuite(func() {
	if !skipCleanup {
		By("Tearing down the kind cluster")
		tearDown(clusterProvider, clusterProxy)
	}
})

func initScheme() *runtime.Scheme {
	sc := runtime.NewScheme()

	_ = corev1.AddToScheme(sc)
	_ = appsv1.AddToScheme(sc)
	_ = rbacv1.AddToScheme(sc)
	_ = v1alpha1.AddToScheme(sc)

	return sc
}

func setupCluster(scheme *runtime.Scheme) (bootstrap.ClusterProvider, framework.ClusterProxy) {
	var clusterProvider bootstrap.ClusterProvider
	kubeconfigPath := ""

	images := []framework.ContainerImage{}
	for _, c := range strings.Split(tryLoadImages, " ") {
		images = append(images, framework.ContainerImage{
			Name:         c,
			LoadBehavior: framework.TryLoadImage,
		})
	}
	for _, c := range strings.Split(mustLoadImages, " ") {
		images = append(images, framework.ContainerImage{
			Name:         c,
			LoadBehavior: framework.MustLoadImage,
		})
	}

	clusterProvider = bootstrap.CreateKindClusterAndLoadImages(context.TODO(), bootstrap.CreateKindClusterAndLoadImagesInput{
		Name:               fmt.Sprintf("e2e-%d", time.Now().Unix()),
		RequiresDockerSock: true,
		Images:             images,
		NodeImage:          nodeImage,
		KindconfigPath:     kindconfigPath,
	})
	Expect(clusterProvider).ToNot(BeNil(), "Failed to create a bootstrap cluster")

	kubeconfigPath = clusterProvider.GetKubeconfigPath()
	Expect(kubeconfigPath).To(BeAnExistingFile(), "Failed to get the kubeconfig file for the bootstrap cluster")

	clusterProxy := framework.NewClusterProxy("bootstrap", kubeconfigPath, scheme)
	Expect(clusterProxy).ToNot(BeNil(), "Failed to get a bootstrap cluster proxy")

	return clusterProvider, clusterProxy
}

func tearDown(clusterProvider bootstrap.ClusterProvider, clusterProxy framework.ClusterProxy) {
	if clusterProxy != nil {
		clusterProxy.Dispose(context.TODO())
	}
	if clusterProvider != nil {
		clusterProvider.Dispose(context.TODO())
	}
}
