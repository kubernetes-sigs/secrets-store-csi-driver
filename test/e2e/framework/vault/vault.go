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
	"context"
	"fmt"

	"k8s.io/client-go/tools/clientcmd"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
	localexec "sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/exec"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/pod"
)

const (
	vaultAuth          = "vault-auth"
	tokenReviewBinding = "role-tokenreview-binding"
	vaultDeployment    = "vault"
	vaultImage         = "registry.hub.docker.com/library/vault:1.5.3"
	vaultService       = "vault"

	policyName = "example-readonly"
	policyFile = "vault/example-readonly.hcl"
	RoleName   = "example-role"
)

var (
	vaultLabels = map[string]string{
		"app": "vault",
	}
	exampleReadonlyPolicy = []byte(`path "secret/data/foo" {
  capabilities = ["read", "list"]
}

path "secret/data/foo1" {
  capabilities = ["read", "list"]
}

path "sys/renew/*" {
  capabilities = ["update"]
}`)
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
	e2e.Byf("%s: Installing %s service account", input.Namespace, vaultAuth)

	Expect(input.Creator.Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vaultAuth,
			Namespace: input.Namespace,
		},
	})).To(Succeed())
}

func installTokenReviewBinding(ctx context.Context, input SetupVaultInput) {
	e2e.Byf("%s: Installing tokenreview clusterrolebinding", input.Namespace)

	Expect(input.Creator.Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: tokenReviewBinding,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:auth-delegator",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      vaultAuth,
				Namespace: input.Namespace,
			},
		},
	})).To(Succeed())
}

func installAndWaitVault(ctx context.Context, input SetupVaultInput) {
	e2e.Byf("%s: Installing vault deployment", input.Namespace)

	replicas := new(int32)
	*replicas = 1
	localConfig := `
api_addr     = "http://127.0.0.1:8200"
cluster_addr = "http://$(POD_IP_ADDR):8201"`

	Expect(input.Creator.Create(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vaultDeployment,
			Namespace: input.Namespace,
			Labels:    vaultLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: vaultLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: vaultLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  vaultDeployment,
							Image: vaultImage,
							Args:  []string{"server", "-dev"},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"IPC_LOCK"},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "vault-port",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: 8200,
								},
								{
									Name:          "cluster-port",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: 8201,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "POD_IP_ADDR",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "status.podIP",
										},
									},
								},
								{
									Name:  "VAULT_LOCAL_CONFIG",
									Value: localConfig,
								},
								{
									Name:  "VAULT_DEV_ROOT_TOKEN_ID",
									Value: "root",
								},
								{
									Name:  "VAULT_ADDR",
									Value: "http://127.0.0.1:8200",
								},
								{
									Name:  "VAULT_TOKEN",
									Value: "root",
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/v1/sys/health",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: 8200,
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	})).To(Succeed())

	pod.WaitForPod(ctx, pod.WaitForPodInput{
		GetLister: input.GetLister,
		Namespace: input.Namespace,
		Labels:    vaultLabels,
	})
}

func installVaultService(ctx context.Context, input SetupVaultInput) {
	e2e.Byf("%s: Installing vault service", input.Namespace)

	Expect(input.Creator.Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vaultService,
			Namespace: input.Namespace,
			Labels:    vaultLabels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: vaultLabels,
			Ports: []corev1.ServicePort{
				{
					Name: "vault-port",
					Port: 8200,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8200,
					},
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	})).To(Succeed())
}

func configureVault(ctx context.Context, input SetupVaultInput) {
	e2e.Byf("%s: Configuring vault service", input.Namespace)

	pods := &corev1.PodList{}
	Expect(input.GetLister.List(ctx, pods, &client.ListOptions{
		Namespace: input.Namespace,
		LabelSelector: labels.SelectorFromValidatedSet(labels.Set(map[string]string{
			"app": "vault",
		})),
	})).To(Succeed(), "Failed to list pods %#v", pods)

	podName := pods.Items[0].Name

	stdout, stderr, err := localexec.KubectlExec(input.KubeconfigPath, podName, input.Namespace, "vault", "auth", "enable", "kubernetes")
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)

	sa := &corev1.ServiceAccount{}
	Expect(input.GetLister.Get(ctx, client.ObjectKey{
		Namespace: input.Namespace,
		Name:      vaultAuth,
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

	stdout, stderr, err = localexec.KubectlExec(input.KubeconfigPath, podName, input.Namespace, "vault", "write", "auth/kubernetes/config",
		"kubernetes_host=https://"+clusterIP,
		"kubernetes_ca_cert="+string(k8sCACert),
		"token_reviewer_jwt="+string(tokenReviewAccountToken))
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = localexec.KubectlExecWithInput(exampleReadonlyPolicy, input.KubeconfigPath, podName, input.Namespace, "vault", "policy", "write", policyName, "-")
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = localexec.KubectlExec(input.KubeconfigPath, podName, input.Namespace, "vault", "write", "auth/kubernetes/role/"+RoleName,
		"bound_service_account_names=secrets-store-csi-driver",
		"bound_service_account_namespaces="+input.Namespace,
		"policies=default,"+policyName,
		"ttl=20m")
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = localexec.KubectlExec(input.KubeconfigPath, podName, input.Namespace, "vault", "kv", "put",
		"secret/foo", "bar=hello")
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)

	stdout, stderr, err = localexec.KubectlExec(input.KubeconfigPath, podName, input.Namespace, "vault", "kv", "put",
		"secret/foo1", "bar=hello1")
	Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
}

type GetAddressInput struct {
	Getter    framework.Getter
	Namespace string
}

func GetAddress(ctx context.Context, input GetAddressInput) string {
	service := &corev1.Service{}
	Expect(input.Getter.Get(ctx, client.ObjectKey{
		Namespace: input.Namespace,
		Name:      vaultService,
	}, service)).To(Succeed(), "Failed to get service %#v", service)

	return fmt.Sprintf("http://%s:8200", service.Spec.ClusterIP)
}
