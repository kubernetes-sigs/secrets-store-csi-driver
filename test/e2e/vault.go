// +build vault

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
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"path/filepath"
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
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/exec"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/vault"
)

// VaultSpecInput is the input for VaultSpec.
type VaultSpecInput struct {
	clusterProxy framework.ClusterProxy
	csiNamespace string
	skipCleanup  bool
	chartPath    string
	manifestsDir string
}

// VaultSpec implements a spec that testing Vault provider
func VaultSpec(ctx context.Context, inputGetter func() VaultSpecInput) {
	var (
		specName      = "provider"
		input         VaultSpecInput
		namespace     *corev1.Namespace
		cancelWatches context.CancelFunc
		cli           client.Client
		codecs        serializer.CodecFactory
	)

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		input = inputGetter()
		Expect(input.clusterProxy).ToNot(BeNil(), "Invalid argument. input.clusterProxy can't be nil when calling %s spec", specName)

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(ctx, specName, input.clusterProxy)

		codecs = serializer.NewCodecFactory(input.clusterProxy.GetScheme())

		cli = input.clusterProxy.GetClient()
	})

	It("test CSI inline volume with pod portability", func() {
		framework.Byf("%s: Installing secretproviderclass", namespace.Name)

		service := &corev1.Service{}
		Expect(cli.Get(ctx, client.ObjectKey{
			Namespace: input.csiNamespace,
			Name:      "vault",
		}, service)).To(Succeed(), "Failed to get service %#v", service)

		data, err := ioutil.ReadFile(filepath.Join(input.manifestsDir, "vault/vault_v1alpha1_secretproviderclass.yaml"))
		Expect(err).To(Succeed())

		buf := new(bytes.Buffer)
		err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
			Namespace    string
			RoleName     string
			VaultAddress string
		}{
			Namespace:    namespace.Name,
			RoleName:     vault.RoleName,
			VaultAddress: fmt.Sprintf("http://%s:8200", service.Spec.ClusterIP),
		})
		Expect(err).To(Succeed())

		obj, _, err := codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
		Expect(err).To(Succeed())

		Expect(cli.Create(ctx, obj)).To(Succeed())

		spc := &spcv1alpha1.SecretProviderClass{}
		Expect(cli.Get(ctx, client.ObjectKey{
			Namespace: namespace.Name,
			Name:      "vault-foo",
		}, spc)).To(Succeed(), "Failed to get secretproviderclass %#v", spc)

		framework.Byf("%s: Installing nginx pod", namespace.Name)

		data, err = ioutil.ReadFile(filepath.Join(input.manifestsDir, "vault/nginx-pod-vault-inline-volume-secretproviderclass.yaml"))
		Expect(err).To(Succeed())

		buf = new(bytes.Buffer)
		err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
			Namespace string
			Image     string
		}{
			Namespace: namespace.Name,
			Image:     "nginx",
		})
		Expect(err).To(Succeed())

		obj, _, err = codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
		Expect(err).To(Succeed())

		Expect(cli.Create(ctx, obj)).To(Succeed())

		framework.Byf("%s: Waiting for nginx pod is running", namespace.Name)

		pod := &corev1.Pod{}
		podName := "nginx-secrets-store-inline"
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

		framework.Byf("%s: Reading secrets from nginx pod", namespace.Name)

		stdout, stderr, err := exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "cat", "/mnt/secrets-store/foo")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello"))

		stdout, stderr, err = exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "cat", "/mnt/secrets-store/foo1")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello1"))
	})

	It("test Sync with K8s secrets", func() {
		framework.Byf("%s: Installing secretproviderclass", namespace.Name)

		service := &corev1.Service{}
		Expect(cli.Get(ctx, client.ObjectKey{
			Namespace: input.csiNamespace,
			Name:      "vault",
		}, service)).To(Succeed(), "Failed to get service %#v", service)

		data, err := ioutil.ReadFile(filepath.Join(input.manifestsDir, "vault/vault_synck8s_v1alpha1_secretproviderclass.yaml"))
		Expect(err).To(Succeed())

		buf := new(bytes.Buffer)
		err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
			Namespace    string
			RoleName     string
			VaultAddress string
		}{
			Namespace:    namespace.Name,
			RoleName:     vault.RoleName,
			VaultAddress: fmt.Sprintf("http://%s:8200", service.Spec.ClusterIP),
		})
		Expect(err).To(Succeed())

		obj, _, err := codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
		Expect(err).To(Succeed())

		Expect(cli.Create(ctx, obj)).To(Succeed())

		spc := &spcv1alpha1.SecretProviderClass{}
		Expect(cli.Get(ctx, client.ObjectKey{
			Namespace: namespace.Name,
			Name:      "vault-foo-sync",
		}, spc)).To(Succeed(), "Failed to get service %#v", service)

		framework.Byf("%s: Installing nginx deployment", namespace.Name)

		data, err = ioutil.ReadFile(filepath.Join(input.manifestsDir, "vault/nginx-deployment-synck8s.yaml"))
		Expect(err).To(Succeed())

		buf = new(bytes.Buffer)
		err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
			Namespace string
			Image     string
		}{
			Namespace: namespace.Name,
			Image:     "nginx",
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

		framework.Byf("%s: Reading secrets from nginx pod's volume", namespace.Name)

		pods := &corev1.PodList{}
		Expect(cli.List(ctx, pods, &client.ListOptions{
			Namespace: namespace.Name,
			LabelSelector: labels.SelectorFromValidatedSet(labels.Set(map[string]string{
				"app": "nginx",
			})),
		})).To(Succeed(), "Failed to list pods %#v", pods)

		podName := pods.Items[0].Name

		stdout, stderr, err := exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "cat", "/mnt/secrets-store/foo")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello"))

		stdout, stderr, err = exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "cat", "/mnt/secrets-store/foo1")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello1"))

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

		pwd, ok := secret.Data["pwd"]
		Expect(ok).To(BeTrue())
		Expect(string(pwd)).To(Equal("hello"))

		// TODO: Check labels of SecretObject
		// l, ok := secret.ObjectMeta.Labels["environment"]
		// Expect(ok).To(BeTrue())
		// Expect(string(l)).To(Equal("test"))

		framework.Byf("%s: Reading environment variable of nginx pod", namespace.Name)

		stdout, stderr, err = exec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "printenv", "SECRET_USERNAME")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello1"))

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

	AfterEach(func() {
		if !input.skipCleanup {
			cleanup(ctx, specName, input.clusterProxy, namespace, cancelWatches)
		}
	})
}
