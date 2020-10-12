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
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/csidriver"
)

// Test suite flags
var (
	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool

	// clusterType is type of Kubernetes cluster
	clusterType string

	// kubeconfigPath is kubeconfig path
	kubeconfigPath string

	// chartPath is helm chart path.
	chartPath string

	// tryLoadImages is a list of image tag if exists in local. loading from local will test faster.
	tryLoadImages string

	// MustLoadImages is a list of image tag that must be in local.
	mustLoadImages string
)

// Test suite global vars
var (
	// clusterProxy allows to interact with the kind cluster to be used for the e2e tests.
	clusterProxy framework.ClusterProxy

	ctx = context.TODO()
)

const (
	defaultClusterType     = "kind"
	defaultKindClusterName = "kind"
	csiNamespace           = "secrets-store-csi-driver"
)

func init() {
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.StringVar(&clusterType, "e2e.cluster-type", defaultClusterType, "type of cluster")
	flag.StringVar(&kubeconfigPath, "e2e.kubeconfig-path", "", "kubeconfig path")
	flag.StringVar(&chartPath, "e2e.chart-path", "", "helm chart path")
	flag.StringVar(&tryLoadImages, "e2e.try-load-images", "", "space separated list of image tag if exists in local")
	flag.StringVar(&mustLoadImages, "e2e.must-load-images", "", "space separated list of image tag that must be in local")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	SetDefaultEventuallyTimeout(15 * time.Minute)
	SetDefaultEventuallyPollingInterval(10 * time.Second)

	RunSpecs(t, "secrets-store-csi-driver")
}

var _ = BeforeSuite(func() {
	By("Setting up the cluster")
	clusterProxy = setupCluster(initScheme())
	Expect(clusterProxy).ToNot(BeNil(), "Failed to get a e2e cluster proxy")

	By("Install CSI driver")
	cli := clusterProxy.GetClient()
	Eventually(func() error {
		return cli.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: csiNamespace,
			},
		})
	}).Should(Succeed())

	csidriver.InstallAndWait(ctx, csidriver.InstallAndWaitInput{
		GetLister:      cli,
		KubeConfigPath: clusterProxy.GetKubeconfigPath(),
		ChartPath:      chartPath,
		Namespace:      csiNamespace,
	})
})

func initScheme() *runtime.Scheme {
	sc := runtime.NewScheme()
	_ = corev1.AddToScheme(sc)
	_ = appsv1.AddToScheme(sc)
	_ = rbacv1.AddToScheme(sc)
	_ = v1alpha1.AddToScheme(sc)

	return sc
}

func setupCluster(scheme *runtime.Scheme) framework.ClusterProxy {
	// TODO: Support loading images for other type of clusters
	if clusterType == defaultClusterType {
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

		Expect(bootstrap.LoadImagesToKindCluster(ctx, bootstrap.LoadImagesToKindClusterInput{
			Name:   defaultKindClusterName,
			Images: images,
		})).To(Succeed())
	}

	Expect(kubeconfigPath).To(BeAnExistingFile(), "Failed to get the kubeconfig file for the cluster")

	return framework.NewClusterProxy("e2e", kubeconfigPath, scheme)
}
