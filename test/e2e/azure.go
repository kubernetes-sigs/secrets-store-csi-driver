// +build azure

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
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	spcv1alpha1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/azure"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/csidriver"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/exec"
)

// AzureSpecInput is the input for AzureSpec.
type AzureSpecInput struct {
	clusterProxy framework.ClusterProxy
	skipCleanup  bool
	chartPath    string
	manifestsDir string
}

// AzureSpec implements a spec that testing Azure provider
func AzureSpec(ctx context.Context, inputGetter func() AzureSpecInput) {
	var (
		specName         = "azure-provider"
		input            AzureSpecInput
		namespace        *corev1.Namespace
		cancelWatches    context.CancelFunc
		cli              client.Client
		codecs           serializer.CodecFactory
		secretName       = "secret1"
		secretVersion    = ""
		secretValue      = "test"
		keyVaultName     = "csi-secrets-store-e2e"
		keyName          = "key1"
		keyVersion       = "7cc095105411491b84fe1b92ebbcf01a"
		catCommand       = "cat"
		image            = "nginx"
		keyValueContains = "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUlJQklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUF4K2FadlhJN2FldG5DbzI3akVScgpheklaQ2QxUlBCQVZuQU1XcDhqY05TQk5MOXVuOVJrenJHOFd1SFBXUXNqQTA2RXRIOFNSNWtTNlQvaGQwMFNRCk1aODBMTlNxYkkwTzBMcWMzMHNLUjhTQ0R1cEt5dkpkb01LSVlNWHQzUlk5R2Ywam1ucHNKOE9WbDFvZlRjOTIKd1RINXYyT2I1QjZaMFd3d25MWlNiRkFnSE1uTHJtdEtwZTVNcnRGU21nZS9SL0J5ZXNscGU0M1FubnpndzhRTwpzU3ZMNnhDU21XVW9WQURLL1MxREU0NzZBREM2a2hGTjF5ZHUzbjVBcnREVGI0c0FjUHdTeXB3WGdNM3Y5WHpnClFKSkRGT0JJOXhSTW9UM2FjUWl0Z0c2RGZibUgzOWQ3VU83M0o3dUFQWUpURG1pZGhrK0ZFOG9lbjZWUG9YRy8KNXdJREFRQUIKLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0t"
		labelValue       = "test"
	)

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		input = inputGetter()
		Expect(input.clusterProxy).ToNot(BeNil(), "Invalid argument. input.clusterProxy can't be nil when calling %s spec", specName)

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, input.clusterProxy)

		codecs = serializer.NewCodecFactory(input.clusterProxy.GetScheme())
		if runtime.GOOS == "windows" {
			catCommand = "powershell.exe cat"
			image = "mcr.microsoft.com/windows/servercore/iis:windowsservercore-ltsc2019"
		}

		cli = input.clusterProxy.GetClient()
	})

	It("test CSI inline volume with pod portability", func() {
		framework.Byf("%s: Installing secretproviderclass", namespace.Name)

		data, err := ioutil.ReadFile(filepath.Join(input.manifestsDir, "azure/azure_v1alpha1_secretproviderclass.yaml"))
		Expect(err).To(Succeed())

		buf := new(bytes.Buffer)
		err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
			Namespace     string
			KeyVaultName  string
			SecretName    string
			SecretVersion string
			KeyName       string
			KeyVersion    string
		}{
			Namespace:     namespace.Name,
			KeyVaultName:  keyVaultName,
			SecretName:    secretName,
			SecretVersion: secretVersion,
			KeyName:       keyName,
			KeyVersion:    keyVersion,
		})
		Expect(err).To(Succeed())

		obj, _, err := codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
		Expect(err).To(Succeed())

		Expect(cli.Create(ctx, obj)).To(Succeed())

		spc := &spcv1alpha1.SecretProviderClass{}
		Expect(cli.Get(ctx, client.ObjectKey{
			Namespace: namespace.Name,
			Name:      "azure",
		}, spc)).To(Succeed(), "Failed to get secretproviderclass %#v", spc)

		framework.Byf("%s: Installing nginx pod", namespace.Name)

		data, err = ioutil.ReadFile(filepath.Join(input.manifestsDir, "azure/nginx-pod-secrets-store-inline-volume-crd.yaml"))
		Expect(err).To(Succeed())

		buf = new(bytes.Buffer)
		err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
			Namespace string
			Image     string
		}{
			Namespace: namespace.Name,
			Image:     image,
		})
		Expect(err).To(Succeed())

		obj, _, err = codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
		Expect(err).To(Succeed())

		Expect(cli.Create(ctx, obj)).To(Succeed())

		framework.Byf("%s: Waiting for nginx pod is running", namespace.Name)

		pod := &corev1.Pod{}
		podName := "nginx-secrets-store-inline-crd"
		Eventually(func() error {
			err := cli.Get(ctx, client.ObjectKey{
				Namespace: namespace.Name,
				Name:      podName,
			}, pod)
			if err != nil {
				return err
			}

			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return nil
				}
			}
			return errors.New("pod is not ready")
		}, framework.WaitTimeout, framework.WaitPolling).Should(Succeed())

		framework.Byf("%s: Reading azure kv secret from pod", namespace.Name)

		stdout, stderr, err := exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, catCommand, "/mnt/secrets-store/"+secretName)
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal(secretValue))

		framework.Byf("%s: Reading azure kv key from pod", namespace.Name)

		stdout, stderr, err = exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, catCommand, "/mnt/secrets-store/"+keyName)
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		encoded := base64.StdEncoding.EncodeToString(bytes.TrimSpace(stdout))
		Expect(err).To(Succeed())
		Expect(encoded).To(Equal(keyValueContains))
	})

	It("test Sync with K8s secrets", func() {
		framework.Byf("%s: Installing secretproviderclass", namespace.Name)

		data, err := ioutil.ReadFile(filepath.Join(input.manifestsDir, "azure/azure_synck8s_v1alpha1_secretproviderclass.yaml"))
		Expect(err).To(Succeed())

		buf := new(bytes.Buffer)
		err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
			Namespace     string
			KeyVaultName  string
			SecretName    string
			SecretVersion string
			KeyName       string
			KeyVersion    string
		}{
			Namespace:     namespace.Name,
			KeyVaultName:  keyVaultName,
			SecretName:    secretName,
			SecretVersion: secretVersion,
			KeyName:       keyName,
			KeyVersion:    keyVersion,
		})
		Expect(err).To(Succeed())

		obj, _, err := codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
		Expect(err).To(Succeed())

		Expect(cli.Create(ctx, obj)).To(Succeed())

		spc := &spcv1alpha1.SecretProviderClass{}
		Expect(cli.Get(ctx, client.ObjectKey{
			Namespace: namespace.Name,
			Name:      "azure-sync",
		}, spc)).To(Succeed(), "Failed to get secretproviderclass %#v", spc)

		framework.Byf("%s: Installing nginx deployment", namespace.Name)

		data, err = ioutil.ReadFile(filepath.Join(input.manifestsDir, "azure/nginx-deployment-synck8s-azure.yaml"))
		Expect(err).To(Succeed())

		buf = new(bytes.Buffer)
		err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
			Namespace string
			Image     string
		}{
			Namespace: namespace.Name,
			Image:     image,
		})
		Expect(err).To(Succeed())

		obj, _, err = codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
		Expect(err).To(Succeed())

		Expect(cli.Create(ctx, obj)).To(Succeed())

		framework.Byf("%s: Waiting for nginx deployment is running", namespace.Name)

		deploy := &appsv1.Deployment{}
		deployName := "nginx-deployment"
		Eventually(func() error {
			err := cli.Get(ctx, client.ObjectKey{
				Namespace: namespace.Name,
				Name:      deployName,
			}, deploy)
			if err != nil {
				return err
			}

			if int(deploy.Status.ReadyReplicas) != 2 {
				return errors.New("ReadyReplicas is not 2")
			}

			return nil
		}, framework.WaitTimeout, framework.WaitPolling).Should(Succeed())

		framework.Byf("%s: Reading secret from pod", namespace.Name)

		pods := &corev1.PodList{}
		Expect(cli.List(ctx, pods, &client.ListOptions{
			Namespace: namespace.Name,
			LabelSelector: labels.SelectorFromValidatedSet(labels.Set(map[string]string{
				"app": "nginx",
			})),
		})).To(Succeed(), "Failed to list pods %#v", pods)

		podName := pods.Items[0].Name

		stdout, stderr, err := exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, catCommand, "/mnt/secrets-store/secretalias")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal(secretValue))

		stdout, stderr, err = exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, catCommand, "/mnt/secrets-store/"+keyName)
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		encoded := base64.StdEncoding.EncodeToString(bytes.TrimSpace(stdout))
		Expect(encoded).To(Equal(keyValueContains))

		framework.Byf("%s: Reading generated secret", namespace.Name)

		secret := &corev1.Secret{}
		Eventually(func() error {
			err := cli.Get(ctx, client.ObjectKey{
				Namespace: namespace.Name,
				Name:      "foosecret",
			}, secret)
			if err != nil {
				return err
			}

			if len(secret.ObjectMeta.OwnerReferences) != 2 {
				return errors.New("OwnerReferences is not 2")
			}

			return nil
		}, framework.WaitTimeout, framework.WaitPolling).Should(Succeed())

		pwd, ok := secret.Data["username"]
		Expect(ok).To(BeTrue())
		Expect(string(pwd)).To(Equal(secretValue))

		l, ok := secret.ObjectMeta.Labels["environment"]
		Expect(ok).To(BeTrue())
		Expect(string(l)).To(Equal(labelValue))

		framework.Byf("%s: Reading environment variable of nginx pod", namespace.Name)

		stdout, stderr, err = exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "printenv", "SECRET_USERNAME")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal(secretValue))

		framework.Byf("%s: Deleting nginx deployment", namespace.Name)

		Expect(cli.Delete(ctx, deploy)).To(Succeed())

		framework.Byf("%s: Waiting secret is deleted", namespace.Name)

		Eventually(func() error {
			err := cli.Get(ctx, client.ObjectKey{
				Namespace: namespace.Name,
				Name:      "foosecret",
			}, secret)
			if err == nil {
				return fmt.Errorf("secret foosecret still exists")
			}

			return nil
		}, framework.WaitTimeout, framework.WaitPolling).Should(Succeed())
	})

	It("test CSI inline volume should fail when no secret provider class in same namespace", func() {
		framework.Byf("%s: Installing nginx deployment", namespace.Name)

		data, err := ioutil.ReadFile(filepath.Join(input.manifestsDir, "azure/nginx-deployment-synck8s-azure.yaml"))
		Expect(err).To(Succeed())

		buf := new(bytes.Buffer)
		err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
			Namespace string
			Image     string
		}{
			Namespace: namespace.Name,
			Image:     image,
		})
		Expect(err).To(Succeed())

		obj, _, err := codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
		Expect(err).To(Succeed())

		Expect(cli.Create(ctx, obj)).To(Succeed())

		framework.Byf("%s: Waiting for nginx deployment is running", namespace.Name)

		deploy := &appsv1.Deployment{}
		deployName := "nginx-deployment"
		Eventually(func() error {
			err := cli.Get(ctx, client.ObjectKey{
				Namespace: namespace.Name,
				Name:      deployName,
			}, deploy)
			if err != nil {
				return err
			}

			if int(deploy.Status.ReadyReplicas) != 0 {
				return errors.New("ReadyReplicas is not 0")
			}

			return nil
		}, framework.WaitTimeout, framework.WaitPolling).Should(Succeed())

		// TODO: Validate event 'FailedMount.*failed to get secretproviderclass negative-test-ns/azure-sync.*not found'"
	})

	It("test CSI inline volume with multiple secret provider class", func() {
		framework.Byf("%s: Installing secretproviderclasses", namespace.Name)

		spcValues := []struct {
			spcName          string
			secretObjectName string
		}{
			{
				spcName:          "azure-spc-0",
				secretObjectName: "foosecret-0",
			},
			{
				spcName:          "azure-spc-1",
				secretObjectName: "foosecret-1",
			},
		}

		for _, spcv := range spcValues {
			data, err := ioutil.ReadFile(filepath.Join(input.manifestsDir, "azure/azure_v1alpha1_multiple_secretproviderclass.yaml"))
			Expect(err).To(Succeed())

			buf := new(bytes.Buffer)
			err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
				Name             string
				Namespace        string
				SecretObjectName string
				KeyVaultName     string
				SecretName       string
				SecretVersion    string
				KeyName          string
				KeyVersion       string
			}{
				Name:             spcv.spcName,
				Namespace:        namespace.Name,
				SecretObjectName: spcv.secretObjectName,
				KeyVaultName:     keyVaultName,
				SecretName:       secretName,
				SecretVersion:    secretVersion,
				KeyName:          keyName,
				KeyVersion:       keyVersion,
			})
			Expect(err).To(Succeed())

			obj, _, err := codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
			Expect(err).To(Succeed())

			Expect(cli.Create(ctx, obj)).To(Succeed())
		}

		for _, spcv := range spcValues {
			spc := &spcv1alpha1.SecretProviderClass{}
			Expect(cli.Get(ctx, client.ObjectKey{
				Namespace: namespace.Name,
				Name:      spcv.spcName,
			}, spc)).To(Succeed(), "Failed to get secretproviderclass %#v", spc)
		}
		framework.Byf("%s: Installing nginx pod", namespace.Name)

		data, err := ioutil.ReadFile(filepath.Join(input.manifestsDir, "azure/nginx-pod-azure-inline-volume-multiple-spc.yaml"))
		Expect(err).To(Succeed())

		buf := new(bytes.Buffer)
		err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
			Namespace string
			Image     string
		}{
			Namespace: namespace.Name,
			Image:     image,
		})
		Expect(err).To(Succeed())

		obj, _, err := codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
		Expect(err).To(Succeed())

		Expect(cli.Create(ctx, obj)).To(Succeed())

		framework.Byf("%s: Waiting for nginx pod is running", namespace.Name)

		pod := &corev1.Pod{}
		podName := "nginx-secrets-store-inline-multiple-crd"
		Eventually(func() error {
			err := cli.Get(ctx, client.ObjectKey{
				Namespace: namespace.Name,
				Name:      podName,
			}, pod)
			if err != nil {
				return err
			}

			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return nil
				}
			}
			return errors.New("pod is not ready")
		}, framework.WaitTimeout, framework.WaitPolling).Should(Succeed())

		framework.Byf("%s: Reading secret from pod", namespace.Name)

		for i, spcv := range spcValues {
			stdout, stderr, err := exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, catCommand, fmt.Sprintf("/mnt/secrets-store-%d/secretalias", i))
			Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
			Expect(strings.TrimSpace(string(stdout))).To(Equal(secretValue))

			stdout, stderr, err = exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, catCommand, fmt.Sprintf("/mnt/secrets-store-%d/%s", i, keyName))
			Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
			encoded := base64.StdEncoding.EncodeToString(bytes.TrimSpace(stdout))
			Expect(encoded).To(Equal(keyValueContains))

			framework.Byf("%s: Reading generated secret", namespace.Name)

			secret := &corev1.Secret{}
			Eventually(func() error {
				err := cli.Get(ctx, client.ObjectKey{
					Namespace: namespace.Name,
					Name:      spcv.secretObjectName,
				}, secret)
				if err != nil {
					return err
				}

				if len(secret.ObjectMeta.OwnerReferences) != 1 {
					return errors.New("OwnerReferences is not 1")
				}

				return nil
			}, framework.WaitTimeout, framework.WaitPolling).Should(Succeed())

			pwd, ok := secret.Data["username"]
			Expect(ok).To(BeTrue())
			Expect(string(pwd)).To(Equal(secretValue))

			l, ok := secret.ObjectMeta.Labels["environment"]
			Expect(ok).To(BeTrue())
			Expect(string(l)).To(Equal(labelValue))

			framework.Byf("%s: Reading environment variable of nginx pod", namespace.Name)

			stdout, stderr, err = exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "printenv", fmt.Sprintf("SECRET_USERNAME_%d", i))
			Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
			Expect(strings.TrimSpace(string(stdout))).To(Equal(secretValue))
		}
	})

	AfterEach(func() {
		if !input.skipCleanup {
			azure.TeardownAzure(ctx, azure.TeardownAzureInput{
				Deleter:        cli,
				Namespace:      namespace.Name,
				ManifestsDir:   input.manifestsDir,
				KubeconfigPath: input.clusterProxy.GetKubeconfigPath(),
			})
			azure.UninstallProvider(ctx, azure.UninstallProviderInput{
				Deleter:   cli,
				Namespace: namespace.Name})
			csidriver.Uninstall(csidriver.UninstallInput{
				KubeConfigPath: input.clusterProxy.GetKubeconfigPath(),
				Namespace:      namespace.Name,
			})

			cleanup(ctx, specName, input.clusterProxy, namespace, cancelWatches)
		}
	})
}
