// copied from https://github.com/kubernetes-sigs/cluster-api/blob/v0.3.6/test/e2e/common.go
// and modified

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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework"
)

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}

func setupSpecNamespace(ctx context.Context, specName string, clusterProxy framework.ClusterProxy) (*corev1.Namespace, context.CancelFunc) {
	Byf("Creating a namespace for hosting the %q test spec", specName)
	namespace, cancelWatches := framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
		Creator:   clusterProxy.GetClient(),
		ClientSet: clusterProxy.GetClientSet(),
		Name:      fmt.Sprintf("%s-%d", specName, time.Now().Unix()),
	})

	return namespace, cancelWatches
}

func cleanup(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, namespace *corev1.Namespace, cancelWatches context.CancelFunc) {
	Byf("Deleting namespace used for hosting the %q test spec", specName)
	framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
		Deleter: clusterProxy.GetClient(),
		Name:    namespace.Name,
	})
	cancelWatches()
}
