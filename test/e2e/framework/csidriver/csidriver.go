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

// Package csidriver is csidriver helper functions for e2e
package csidriver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"

	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework"
)

const (
	csiDriverReleaseName   = "csi-secrets-store"
	csiDriverDaemonSetName = csiDriverReleaseName + "-secrets-store-csi-driver"
)

type InstallInput struct {
	KubeConfigPath string
	ChartPath      string
	Namespace      string
}

func Install(input InstallInput) {
	framework.Byf("%s: Installing csi-driver by helm chart, chart path: %s", input.Namespace, input.ChartPath)

	chart, err := loader.Load(input.ChartPath)
	Expect(err).To(Succeed())

	actionConfig := new(action.Configuration)

	Expect(actionConfig.Init(kube.GetConfig(input.KubeConfigPath, "", csiDriverReleaseName), input.Namespace, os.Getenv("HELM_DRIVER"), helmDebug)).ToNot(HaveOccurred(), "Failed to initialize the helm client %q")

	i := action.NewInstall(actionConfig)
	i.ReleaseName = csiDriverReleaseName
	i.Namespace = input.Namespace
	i.Wait = true
	i.Timeout = framework.HelmTimeout

	vals := make(map[string]interface{})
	switch runtime.GOOS {
	case "windows":
		vals["windows.image.pullPolicy"] = "IfNotPresent"
		vals["windows.image.repository"] = "$(REGISTRY)/$(IMAGE_NAME)"
		vals["windows.image.tag"] = "e2e"
		vals["windows.enabled"] = true
		vals["linux.enabled"] = false
	case "linux":
		vals["linux.image.pullPolicy"] = "IfNotPresent"
		vals["linux.image.repository"] = "secrets-store-csi"
		vals["linux.image.tag"] = "e2e"
	}

	_, err = i.Run(chart, vals)
	Expect(err).To(Succeed())
}

type WaitInput struct {
	Getter    framework.Getter
	Namespace string
}

func Wait(ctx context.Context, input WaitInput) {
	framework.Byf("%s: Waiting for csi-driver pod is running", input.Namespace)

	Eventually(func() error {
		ds := &appsv1.DaemonSet{}
		err := input.Getter.Get(ctx, client.ObjectKey{
			Namespace: input.Namespace,
			Name:      csiDriverDaemonSetName,
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

type InstallAndWaitInput struct {
	KubeConfigPath string
	ChartPath      string
	Namespace      string

	Getter framework.Getter
}

func InstallAndWait(ctx context.Context, input InstallAndWaitInput) {
	Install(InstallInput{
		KubeConfigPath: input.KubeConfigPath,
		ChartPath:      input.ChartPath,
		Namespace:      input.Namespace,
	})

	Wait(ctx, WaitInput{
		Getter:    input.Getter,
		Namespace: input.Namespace,
	})
}

type UninstallInput struct {
	KubeConfigPath string
	Namespace      string
}

func Uninstall(input UninstallInput) {
	framework.Byf("%s: Uninstalling csi-driver chart", input.Namespace)

	actionConfig := new(action.Configuration)

	Expect(actionConfig.Init(kube.GetConfig(input.KubeConfigPath, "", csiDriverReleaseName), input.Namespace, os.Getenv("HELM_DRIVER"), helmDebug)).ToNot(HaveOccurred(), "Failed to initialize the helm client %q")

	i := action.NewUninstall(actionConfig)
	i.Timeout = framework.HelmTimeout

	_, err := i.Run(csiDriverReleaseName)
	Expect(err).To(Succeed())
}

func helmDebug(format string, v ...interface{}) {
	format = fmt.Sprintf("[helm] %s\n", format)
	klog.Info(fmt.Sprintf(format, v...))
}
