package vault

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"text/template"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e"
)

func testSyncWithK8sSecrets() {
	deploymentYAML := "../tests/nginx-deployment-synck8s.yaml"
	secretName := "foosecret"

	It("create deployment", func() {
		stdout, stderr, err := e2e.Kubectl("get", "service", "vault", "--namespace", e2e.Namespace, "-o", "jsonpath={.spec.clusterIP}")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		url := fmt.Sprintf("http://%s:8200", strings.TrimSpace(string(stdout)))

		tf, err := ioutil.ReadFile("../tests/vault_synck8s_v1alpha1_secretproviderclass.yaml")
		Expect(err).NotTo(HaveOccurred())
		tmpl := template.Must(template.New("").Parse(string(tf)))
		buf := bytes.NewBuffer(nil)
		err = tmpl.Execute(buf, struct {
			RoleName     string
			VaultAddress string
		}{
			RoleName:     e2e.RoleName,
			VaultAddress: url,
		})
		Expect(err).NotTo(HaveOccurred())

		stdout, stderr, err = e2e.KubectlWithInput(buf.Bytes(), "apply", "--namespace", e2e.Namespace, "-f", "-")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		stdout, stderr, err = e2e.Kubectl("get", "secretproviderclasses.secrets-store.csi.x-k8s.io/vault-foo-sync", "--namespace", e2e.Namespace)
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		stdout, stderr, err = e2e.KubectlWithInput(buf.Bytes(), "apply", "--namespace", e2e.Namespace, "-f", deploymentYAML)
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		deployName := "nginx-deployment"
		Eventually(func() error {
			stdout, stderr, err = e2e.Kubectl("get", "deploy/"+deployName, "--namespace", e2e.Namespace, "-o", "json")
			if err != nil {
				return fmt.Errorf("%w, stdout: %s, stderr: %s", err, stdout, stderr)
			}

			var deploy appsv1.Deployment
			err = json.Unmarshal(stdout, &deploy)
			if err != nil {
				return fmt.Errorf("%w, stdout: %s, stderr: %s", err, stdout, stderr)
			}

			replicas := int32(1)
			if deploy.Spec.Replicas != nil {
				replicas = *deploy.Spec.Replicas
			}

			if replicas != deploy.Status.ReadyReplicas {
				return fmt.Errorf(
					"the number of replicas of Deployment %s/%s should be %d: %d",
					deploy.Namespace, deploy.Name, replicas, deploy.Status.ReadyReplicas,
				)
			}

			return nil
		}).Should(Succeed())
	})

	It("read secret from pod, read K8s secret, read env var, check secret ownerReferences", func() {
		stdout, stderr, err := e2e.Kubectl("get", "pod", "-l", "app=nginx", "--namespace", e2e.Namespace, "-o", "jsonpath={.items[0].metadata.name}")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		podName := strings.TrimSpace(string(stdout))

		stdout, stderr, err = e2e.Kubectl("exec", "-it", podName, "--", "cat", "/mnt/secrets-store/foo")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello"))

		stdout, stderr, err = e2e.Kubectl("exec", "-it", podName, "--", "cat", "/mnt/secrets-store/foo1")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello1"))

		stdout, stderr, err = e2e.Kubectl("get", "secret", secretName, "--namespace", e2e.Namespace, "-o", "jsonpath={.data.pwd}")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		result, err := base64.StdEncoding.DecodeString(string(stdout))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(result)).To(Equal("hello"))

		stdout, stderr, err = e2e.Kubectl("exec", "-it", podName, "--", "printenv", "SECRET_USERNAME")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello1"))

		stdout, stderr, err = e2e.Kubectl("get", "secret", secretName, "--namespace", e2e.Namespace, "-o", "json")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var secret corev1.Secret
		err = json.Unmarshal(stdout, &secret)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(secret.ObjectMeta.OwnerReferences)).To(Equal(2))
	})

	It("delete deployment, check secret is deleted", func() {
		stdout, stderr, err := e2e.Kubectl("delete", "--namespace", e2e.Namespace, "-f", deploymentYAML)
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		Eventually(func() error {
			stdout, stderr, err = e2e.Kubectl("get", "secret", secretName)
			if err == nil {
				return fmt.Errorf("secret foosecret still exists, stdout: %s, stderr: %s", stdout, stderr)
			}
			return nil
		}).Should(Succeed())
	})
}
