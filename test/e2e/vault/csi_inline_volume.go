package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"text/template"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e"
)

func testCSIInlineVolume() {
	It("deploy vault secretproviderclass crd", func() {
		stdout, stderr, err := e2e.Kubectl("get", "service", "vault", "--namespace", e2e.Namespace, "-o", "jsonpath={.spec.clusterIP}")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		url := fmt.Sprintf("http://%s:8200", strings.TrimSpace(string(stdout)))

		tf, err := ioutil.ReadFile("../tests/vault_v1alpha1_secretproviderclass.yaml")
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

		stdout, stderr, err = e2e.Kubectl("get", "secretproviderclasses.secrets-store.csi.x-k8s.io/vault-foo", "--namespace", e2e.Namespace)
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
	})

	It("CSI inline volume test with pod portability", func() {
		tf, err := ioutil.ReadFile("../tests/nginx-pod-vault-inline-volume-secretproviderclass.yaml")
		Expect(err).NotTo(HaveOccurred())
		tmpl := template.Must(template.New("").Parse(string(tf)))
		buf := bytes.NewBuffer(nil)
		err = tmpl.Execute(buf, struct {
			ContainerImage string
		}{
			ContainerImage: e2e.ContainerImage,
		})
		Expect(err).NotTo(HaveOccurred())

		stdout, stderr, err := e2e.KubectlWithInput(buf.Bytes(), "apply", "--namespace", e2e.Namespace, "-f", "-")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

		podName := "nginx-secrets-store-inline"
		Eventually(func() error {
			stdout, stderr, err = e2e.Kubectl("get", "pod/"+podName, "--namespace", e2e.Namespace, "-o", "json")
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

		stdout, stderr, err = e2e.Kubectl("exec", "-it", podName, "--", "cat", "/mnt/secrets-store/foo")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello"))

		stdout, stderr, err = e2e.Kubectl("exec", "-it", podName, "--", "cat", "/mnt/secrets-store/foo1")
		Expect(err).NotTo(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello1"))
	})
}
