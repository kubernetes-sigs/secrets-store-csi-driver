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

package vault

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"path/filepath"
	"text/template"

	"k8s.io/client-go/tools/clientcmd"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/exec"
)

const (
	vaultAuthServiceAccountFile = "vault/vault-auth.yaml"
	tokenReviewBindingFile      = "vault/tokenreview-binding.yaml"
	vaultDeploymentFile         = "vault/vault-deployment.yaml"
	vaultServiceFile            = "vault/vault-service.yaml"
	policyName                  = "example-readonly"
	policyFile                  = "vault/example-readonly.hcl"
	RoleName                    = "example-role"
)

type SetupVaultInput struct {
	Creator        framework.Creator
	GetLister      framework.GetLister
	Namespace      string
	ManifestsDir   string
	KubeconfigPath string
}

func SetupVault(ctx context.Context, input SetupVaultInput) {
	installServiceAccount(ctx, input)
	installTokenReviewBinding(ctx, input)
	installAndWaitVault(ctx, input)
	installVaultService(ctx, input)
	configureVault(ctx, input)
}

func installServiceAccount(ctx context.Context, input SetupVaultInput) {
	framework.Byf("%s: Installing vault-auth service account", input.Namespace)

	installYAML(ctx, filepath.Join(input.ManifestsDir, vaultAuthServiceAccountFile), input)
}

func installTokenReviewBinding(ctx context.Context, input SetupVaultInput) {
	framework.Byf("%s: Installing tokenreview clusterrolebinding", input.Namespace)

	installYAML(ctx, filepath.Join(input.ManifestsDir, tokenReviewBindingFile), input)
}

func installAndWaitVault(ctx context.Context, input SetupVaultInput) {
	framework.Byf("%s: Installing vault deployment", input.Namespace)

	installYAML(ctx, filepath.Join(input.ManifestsDir, vaultDeploymentFile), input)

	framework.Byf("%s: Waiting for vault pod is running", input.Namespace)

	Eventually(func() error {
		deploy := &appsv1.Deployment{}
		err := input.GetLister.Get(ctx, client.ObjectKey{
			Namespace: input.Namespace,
			Name:      "vault",
		}, deploy)
		if err != nil {
			return err
		}

		if int(deploy.Status.ReadyReplicas) != 1 {
			return errors.New("ReadyReplicas is not 1")
		}
		return nil
	}, framework.WaitTimeout, framework.WaitPolling).Should(Succeed())
}

func installVaultService(ctx context.Context, input SetupVaultInput) {
	framework.Byf("%s: Installing vault service", input.Namespace)

	installYAML(ctx, filepath.Join(input.ManifestsDir, vaultServiceFile), input)
}

func configureVault(ctx context.Context, input SetupVaultInput) {
	framework.Byf("%s: Configuring vault service", input.Namespace)

	pods := &corev1.PodList{}
	Expect(input.GetLister.List(ctx, pods, &client.ListOptions{
		Namespace: input.Namespace,
		LabelSelector: labels.SelectorFromValidatedSet(labels.Set(map[string]string{
			"app": "vault",
		})),
	})).To(Succeed(), "Failed to list pods %#v", pods)

	podName := pods.Items[0].Name

	stdout, stderr, err := exec.KubectlExec(input.KubeconfigPath, podName, input.Namespace, "vault", "auth", "enable", "kubernetes")
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)

	sa := &corev1.ServiceAccount{}
	Expect(input.GetLister.Get(ctx, client.ObjectKey{
		Namespace: input.Namespace,
		Name:      "vault-auth",
	}, sa)).To(Succeed(), "Failed to get serviceaccount %#v", sa)

	secretName := sa.Secrets[0].Name
	secret := &corev1.Secret{}
	Expect(input.GetLister.Get(ctx, client.ObjectKey{
		Namespace: input.Namespace,
		Name:      secretName,
	}, secret)).To(Succeed(), "Failed to get secret %#v", secret)

	token, ok := secret.Data["token"]
	Expect(ok).To(BeTrue())
	tokenReviewAccountToken := token
	// tokenReviewAccountToken, err := base64.StdEncoding.DecodeString(string(token))
	// Expect(err).To(Succeed())

	service := &corev1.Service{}
	Expect(input.GetLister.Get(ctx, client.ObjectKey{
		Namespace: "default",
		Name:      "kubernetes",
	}, service)).To(Succeed(), "Failed to get service %#v", service)

	clusterIP := service.Spec.ClusterIP

	config, err := clientcmd.LoadFromFile(input.KubeconfigPath)
	Expect(err).To(Succeed())

	var k8sCACert []byte
	for _, v := range config.Clusters {
		k8sCACert = v.CertificateAuthorityData
		// k8sCACert, err = base64.StdEncoding.DecodeString(string(v.CertificateAuthorityData))
		// Expect(err).To(Succeed())
		// Should have only one cluster entry
		break
	}

	stdout, stderr, err = exec.KubectlExec(input.KubeconfigPath, podName, input.Namespace, "vault", "write", "auth/kubernetes/config",
		"kubernetes_host=https://"+clusterIP,
		"kubernetes_ca_cert="+string(k8sCACert),
		"token_reviewer_jwt="+string(tokenReviewAccountToken))
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)

	data, err := ioutil.ReadFile(filepath.Join(input.ManifestsDir, policyFile))
	Expect(err).To(Succeed())

	stdout, stderr, err = exec.KubectlExecWithInput(data, input.KubeconfigPath, podName, input.Namespace, "vault", "policy", "write", policyName, "-")
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = exec.KubectlExec(input.KubeconfigPath, podName, input.Namespace, "vault", "write", "auth/kubernetes/role/"+RoleName,
		"bound_service_account_names=secrets-store-csi-driver",
		"bound_service_account_namespaces="+input.Namespace,
		"policies=default,"+policyName,
		"ttl=20m")
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = exec.KubectlExec(input.KubeconfigPath, podName, input.Namespace, "vault", "kv", "put",
		"secret/foo", "bar=hello")
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = exec.KubectlExec(input.KubeconfigPath, podName, input.Namespace, "vault", "kv", "put",
		"secret/foo1", "bar=hello1")
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
}

func installYAML(ctx context.Context, file string, input SetupVaultInput) {
	data, err := ioutil.ReadFile(file)
	Expect(err).To(Succeed())

	buf := new(bytes.Buffer)
	err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
		Namespace string
	}{
		input.Namespace,
	})
	Expect(err).To(Succeed())

	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
	Expect(err).To(Succeed())

	Expect(input.Creator.Create(ctx, obj)).To(Succeed())
}

type TeardownVaultInput struct {
	Deleter        framework.Deleter
	GetLister      framework.GetLister
	Namespace      string
	ManifestsDir   string
	KubeconfigPath string
}

func TeardownVault(ctx context.Context, input TeardownVaultInput) {
	// Delete only cluster-wide resources, other resources are deleted by deleting namespace
	uninstallTokenReviewBinding(ctx, input)
}

func uninstallTokenReviewBinding(ctx context.Context, input TeardownVaultInput) {
	framework.Byf("%s: Installing tokenreview clusterrolebinding", input.Namespace)

	uninstallYAML(ctx, filepath.Join(input.ManifestsDir, tokenReviewBindingFile), input)
}

func uninstallYAML(ctx context.Context, file string, input TeardownVaultInput) {
	data, err := ioutil.ReadFile(file)
	Expect(err).To(Succeed())

	buf := new(bytes.Buffer)
	err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
		Namespace string
	}{
		input.Namespace,
	})
	Expect(err).To(Succeed())

	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
	Expect(err).To(Succeed())

	Expect(input.Deleter.Delete(ctx, obj)).To(Succeed())
}
