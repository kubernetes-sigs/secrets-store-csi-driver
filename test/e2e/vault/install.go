package vault

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e"
)

func testInstallVaultProvider() {
	It("install vault provider", func() {
		stdout, stderr, err := e2e.Kubectl("apply", "--namespace", e2e.Namespace, "-f", providerYAML)
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		stdout, stderr, err = e2e.Kubectl("get", "pod", "--namespace", e2e.Namespace, "-l", "app=csi-secrets-store-provider-vault", "-o", "jsonpath={.items[0].metadata.name}")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		vaultProviderPod := string(stdout)
		Eventually(func() error {
			stdout, stderr, err = e2e.Kubectl("get", "pod/"+vaultProviderPod, "--namespace", e2e.Namespace, "-o", "json")
			if err != nil {
				return fmt.Errorf("%w, stdout: %s, stderr: %s", err, stdout, stderr)
			}

			var pod corev1.Pod
			err = json.Unmarshal(stdout, &pod)
			if err != nil {
				return fmt.Errorf("%w, stdout: %s, stderr: %s", err, stdout, stderr)
			}

			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return nil
				}
			}
			return errors.New("pod is not yet ready")
		}).Should(Succeed())

		stdout, stderr, err = e2e.Kubectl("get", "pod/"+string(vaultProviderPod), "--namespace", e2e.Namespace)
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	})

	It("install vault service account", func() {
		stdout, stderr, err := e2e.Kubectl("create", "serviceaccount", "vault-auth")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		clusterRoleBinding := `
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: role-tokenreview-binding
  namespace: default
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: ServiceAccount
  name: vault-auth
  namespace: default
`
		stdout, stderr, err = e2e.KubectlWithInput([]byte(clusterRoleBinding), "create", "-f", "-")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	})

	It("install vault", func() {
		stdout, stderr, err := e2e.Kubectl("apply", "-f", "../tests/vault.yaml")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		stdout, stderr, err = e2e.Kubectl("get", "pod", "--namespace", e2e.Namespace, "-l", "app=vault", "-o", "jsonpath={.items[0].metadata.name}")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		vaultPod := string(stdout)
		Eventually(func() error {
			stdout, stderr, err = e2e.Kubectl("get", "pod/"+vaultPod, "--namespace", e2e.Namespace, "-o", "json")
			if err != nil {
				return fmt.Errorf("%w, stdout: %s, stderr: %s", err, stdout, stderr)
			}

			var pod corev1.Pod
			err = json.Unmarshal(stdout, &pod)
			if err != nil {
				return fmt.Errorf("%w, stdout: %s, stderr: %s", err, stdout, stderr)
			}

			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					return nil
				}
			}
			return errors.New("pod is not ready")
		}).Should(Succeed())
	})

	It("setup vault", func() {
		stdout, stderr, err := e2e.Kubectl("get", "pod", "--namespace", e2e.Namespace, "-l", "app=vault", "-o", "jsonpath={.items[0].metadata.name}")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		vaultPod := string(stdout)

		stdout, stderr, err = e2e.Kubectl("exec", "-it", vaultPod, "--", "vault", "auth", "enable", "kubernetes")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		stdout, stderr, err = e2e.Kubectl("get", "serviceaccount", "vault-auth", "-o", "go-template={{ (index .secrets 0).name }}")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		secretName := string(stdout)

		stdout, stderr, err = e2e.Kubectl("get", "secret", secretName, "-o", "go-template={{ .data.token }}")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		trAccountToken, err := base64.StdEncoding.DecodeString(string(stdout))
		Expect(err).NotTo(HaveOccurred())

		stdout, stderr, err = e2e.Kubectl("get", "svc", "kubernetes", "-o", "go-template={{ .spec.clusterIP }}")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		k8sHost := fmt.Sprintf("https://" + string(stdout))

		stdout, stderr, err = e2e.Kubectl("config", "view", "--raw", "-o", "go-template={{ range .clusters }}{{ if eq .name \"kind-kind\" }}{{ index .cluster \"certificate-authority-data\" }}{{ end }}{{ end }}")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		k8sCACert, err := base64.StdEncoding.DecodeString(string(stdout))
		Expect(err).NotTo(HaveOccurred())

		stdout, stderr, err = e2e.Kubectl("exec", "-it", vaultPod, "--", "vault", "write", "auth/kubernetes/config",
			"kubernetes_host="+k8sHost,
			"kubernetes_ca_cert="+string(k8sCACert),
			"token_reviewer_jwt="+string(trAccountToken))
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		policyName := "example-readonly"
		policy := []byte(`
path "secret/data/foo" {
  capabilities = ["read", "list"]
}

path "secret/data/foo1" {
  capabilities = ["read", "list"]
}

path "sys/renew/*" {
  capabilities = ["update"]
}
`)
		stdout, stderr, err = e2e.KubectlWithInput(policy, "exec", "-it", vaultPod, "--", "vault", "policy", "write", policyName, "-")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		stdout, stderr, err = e2e.Kubectl("exec", "-it", vaultPod, "--", "vault", "write", "auth/kubernetes/role/"+e2e.RoleName,
			"bound_service_account_names=secrets-store-csi-driver",
			"bound_service_account_namespaces="+e2e.Namespace,
			"policies=default,"+policyName,
			"ttl=20m")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		stdout, stderr, err = e2e.Kubectl("exec", "-it", vaultPod, "--", "vault", "kv", "put",
			"secret/foo", "bar=hello")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		stdout, stderr, err = e2e.Kubectl("exec", "-it", vaultPod, "--", "vault", "kv", "put",
			"secret/foo1", "bar=hello1")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

	})

	It("secretproviderclasses crd is established", func() {
		Eventually(func() error {
			stdout, stderr, err := e2e.Kubectl("get", "crd/secretproviderclasses.secrets-store.csi.x-k8s.io", "-o", "json")
			if err != nil {
				return fmt.Errorf("%w, stdout: %s, stderr: %s", err, stdout, stderr)
			}

			var crd apiextensions.CustomResourceDefinition
			err = json.Unmarshal(stdout, &crd)
			if err != nil {
				return fmt.Errorf("%w, stdout: %s, stderr: %s", err, stdout, stderr)
			}

			for _, cond := range crd.Status.Conditions {
				if cond.Type == apiextensions.NamesAccepted && cond.Status == apiextensions.ConditionTrue {
					return nil
				}
			}
			return errors.New("crd is not yet ready")
		}).Should(Succeed())
	})
}
