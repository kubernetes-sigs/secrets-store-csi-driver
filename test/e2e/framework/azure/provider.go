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

// Package azure is helper functions for e2e
package azure

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"runtime"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework"
)

var providerAzureYAML = "https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/deployment/provider-azure-installer.yaml"

const (
	providerAzureDaemonSetName = "csi-secrets-store-provider-azure"
)

type InstallProviderInput struct {
	Creator   framework.Creator
	Namespace string
}

func InstallProvider(ctx context.Context, input InstallProviderInput) {
	framework.Byf("%s: Installing azure provider", input.Namespace)

	if runtime.GOOS == "windows" {
		providerAzureYAML = "https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/deployment/provider-azure-installer-windows.yaml"
	}

	resp := &http.Response{}
	var err error
	Eventually(func() error {
		resp, err = http.Get(providerAzureYAML)
		return err
	}, framework.WaitTimeout, framework.WaitPolling).Should(Succeed())
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
			schema.GroupVersion{Group: corev1.SchemeGroupVersion.Group, Version: corev1.SchemeGroupVersion.Version},
		}

		resourceDecoder := scheme.Codecs.DecoderToVersion(scheme.Codecs.UniversalDeserializer(), gvs)

		obj, _, err := resourceDecoder.Decode(data, nil, nil)
		Expect(err).To(Succeed())

		switch o := obj.(type) {
		case *appsv1.DaemonSet:
			o.Namespace = input.Namespace
		case *corev1.ServiceAccount:
			o.Namespace = input.Namespace
		}

		Eventually(func() error {
			return input.Creator.Create(ctx, obj)
		}, framework.CreateTimeout, framework.CreatePolling).Should(Succeed())
	}
}

type WaitProviderInput struct {
	Getter    framework.Getter
	Namespace string
}

func WaitProvider(ctx context.Context, input WaitProviderInput) {
	framework.Byf("%s: Waiting for azure-provider pod is running", input.Namespace)

	Eventually(func() error {
		ds := &appsv1.DaemonSet{}
		err := input.Getter.Get(ctx, client.ObjectKey{
			Namespace: input.Namespace,
			Name:      providerAzureDaemonSetName,
		}, ds)
		if err != nil {
			return err
		}

		if int(ds.Status.NumberReady) != 1 {
			return errors.New("NumberReady is not 1")
		}
		return nil
	}, framework.WaitTimeout, framework.WaitPolling).Should(Succeed())
}

type InstallAndWaitProviderInput struct {
	Creator   framework.Creator
	Getter    framework.Getter
	Namespace string
}

func InstallAndWaitProvider(ctx context.Context, input InstallAndWaitProviderInput) {
	InstallProvider(ctx, InstallProviderInput{
		Creator:   input.Creator,
		Namespace: input.Namespace,
	})

	WaitProvider(ctx, WaitProviderInput{
		Getter:    input.Getter,
		Namespace: input.Namespace,
	})
}

type UninstallProviderInput struct {
	Deleter   framework.Deleter
	Namespace string
}

func UninstallProvider(ctx context.Context, input UninstallProviderInput) {
	framework.Byf("%s: Uninstalling azure provider", input.Namespace)

	resp, err := http.Get(providerAzureYAML)
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
		}, framework.DeleteTimeout, framework.DeletePolling).Should(Succeed())
	}
}
