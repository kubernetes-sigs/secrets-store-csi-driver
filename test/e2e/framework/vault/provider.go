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

// Package vault is helper functions for e2e
package vault

import (
	"bufio"
	"context"
	"io"
	"net/http"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	localexec "sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/exec"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/pod"
)

const (
	providerYAML = "https://raw.githubusercontent.com/hashicorp/secrets-store-csi-driver-provider-vault/master/deployment/provider-vault-installer.yaml"
)

type InstallProviderInput struct {
	Creator        framework.Creator
	Namespace      string
	KubeconfigPath string
}

func InstallProvider(ctx context.Context, input InstallProviderInput) {
	e2e.Byf("%s: Installing vault provider", input.Namespace)

	stdout, stderr, err := localexec.KubectlApply(input.KubeconfigPath, input.Namespace, providerYAML)
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
}

type InstallAndWaitProviderInput struct {
	Creator        framework.Creator
	GetLister      framework.GetLister
	Namespace      string
	KubeconfigPath string
}

func InstallAndWaitProvider(ctx context.Context, input InstallAndWaitProviderInput) {
	InstallProvider(ctx, InstallProviderInput{
		Creator:   input.Creator,
		Namespace: input.Namespace,
	})

	pod.WaitForPod(ctx, pod.WaitForPodInput{
		GetLister: input.GetLister,
		Namespace: input.Namespace,
		Labels: map[string]string{
			"app": "csi-secrets-store-provider-vault",
		},
	})
}

type UninstallProviderInput struct {
	Deleter   framework.Deleter
	Namespace string
}

func UninstallProvider(ctx context.Context, input UninstallProviderInput) {
	e2e.Byf("%s: Uninstalling vault provider", input.Namespace)

	resp, err := http.Get(providerYAML)
	Expect(err).To(Succeed())
	defer resp.Body.Close()

	y := yaml.NewYAMLReader(bufio.NewReader(resp.Body))
	for {
		data, err := y.Read()
		if err == io.EOF {
			return
		}
		Expect(err).To(Succeed())

		gvs := schema.GroupVersions{
			schema.GroupVersion{Group: appsv1.SchemeGroupVersion.Group, Version: appsv1.SchemeGroupVersion.Version},
		}

		resourceDecoder := scheme.Codecs.DecoderToVersion(scheme.Codecs.UniversalDeserializer(), gvs)

		obj, _, err := resourceDecoder.Decode(data, nil, nil)
		Expect(err).To(Succeed())

		providerObj := obj.(*appsv1.DaemonSet)
		providerObj.Namespace = input.Namespace

		Eventually(func() error {
			return input.Deleter.Delete(ctx, providerObj)
		}).Should(Succeed())
	}
}
