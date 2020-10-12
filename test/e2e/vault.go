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
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	spcv1alpha1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"
	localexec "sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/exec"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/vault"
)

// VaultSpecInput is the input for VaultSpec.
type VaultSpecInput struct {
	clusterProxy framework.ClusterProxy
	skipCleanup  bool
	chartPath    string
}

// VaultSpec implements a spec that testing Vault provider
func VaultSpec(ctx context.Context, inputGetter func() VaultSpecInput) {
	var (
		specName      = "vault-provider"
		input         VaultSpecInput
		namespace     *corev1.Namespace
		cancelWatches context.CancelFunc
		cli           client.Client
	)

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		input = inputGetter()
		Expect(input.clusterProxy).ToNot(BeNil(), "Invalid argument. input.clusterProxy can't be nil when calling %s spec", specName)

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
			Creator:   input.clusterProxy.GetClient(),
			ClientSet: input.clusterProxy.GetClientSet(),
			Name:      fmt.Sprintf("%s-%s", specName, util.RandomString(6)),
			LogFolder: filepath.Join("resources", "clusters", input.clusterProxy.GetName()),
		})

		cli = input.clusterProxy.GetClient()
	})

	It("test CSI inline volume with pod portability", func() {
		e2e.Byf("%s: Installing secretproviderclass", namespace.Name)

		vaultAddress := vault.GetAddress(ctx, vault.GetAddressInput{cli, csiNamespace})

		spcName := "vault-foo"
		Expect(cli.Create(ctx, &spcv1alpha1.SecretProviderClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      spcName,
				Namespace: namespace.Name,
			},
			Spec: spcv1alpha1.SecretProviderClassSpec{
				Provider: "vault",
				Parameters: map[string]string{
					"roleName":           vault.RoleName,
					"vaultAddress":       vaultAddress,
					"vaultSkipTLSVerify": "true",
					"objects": `array:
  - |
    objectPath: "/foo"
    objectName: "bar"
    objectVersion: ""
  - |
    objectPath: "/foo1"
    objectName: "bar"
    objectVersion: ""`,
				},
			},
		})).To(Succeed())

		spc := &spcv1alpha1.SecretProviderClass{}
		Expect(cli.Get(ctx, client.ObjectKey{
			Namespace: namespace.Name,
			Name:      spcName,
		}, spc)).To(Succeed(), "Failed to get secretproviderclass %#v", spc)

		e2e.Byf("%s: Installing nginx pod", namespace.Name)

		podName := "nginx-secrets-store-inline"
		readOnly := new(bool)
		*readOnly = true

		Expect(cli.Create(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace.Name,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            "nginx",
						Image:           "nginx",
						ImagePullPolicy: corev1.PullIfNotPresent,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "secrets-store-inline",
								MountPath: "/mnt/secrets-store",
								ReadOnly:  true,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "secrets-store-inline",
						VolumeSource: corev1.VolumeSource{
							CSI: &corev1.CSIVolumeSource{
								Driver:   "secrets-store.csi.k8s.io",
								ReadOnly: readOnly,
								VolumeAttributes: map[string]string{
									"secretProviderClass": spcName,
								},
							},
						},
					},
				},
			},
		})).To(Succeed())

		e2e.Byf("%s: Waiting for nginx pod is running", namespace.Name)

		pod := &corev1.Pod{}
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
		}).Should(Succeed())

		e2e.Byf("%s: Reading secrets from nginx pod", namespace.Name)

		stdout, stderr, err := localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "cat", "/mnt/secrets-store/foo")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello"))

		stdout, stderr, err = localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "cat", "/mnt/secrets-store/foo1")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello1"))
	})

	It("test Sync with K8s secrets", func() {
		e2e.Byf("%s: Installing secretproviderclass", namespace.Name)

		vaultAddress := vault.GetAddress(ctx, vault.GetAddressInput{cli, csiNamespace})

		spcName := "vault-foo-sync"
		Expect(cli.Create(ctx, &spcv1alpha1.SecretProviderClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      spcName,
				Namespace: namespace.Name,
			},
			Spec: spcv1alpha1.SecretProviderClassSpec{
				Provider: "vault",
				SecretObjects: []*spcv1alpha1.SecretObject{
					{
						SecretName: "foosecret",
						Type:       "Opaque",
						Labels: map[string]string{
							"environment": "test",
						},
						Data: []*spcv1alpha1.SecretObjectData{
							{
								ObjectName: "foo",
								Key:        "pwd",
							},
							{
								ObjectName: "foo1",
								Key:        "username",
							},
						},
					},
				},
				Parameters: map[string]string{
					"roleName":           vault.RoleName,
					"vaultAddress":       vaultAddress,
					"vaultSkipTLSVerify": "true",
					"objects": `array:
  - |
    objectPath: "/foo"
    objectName: "bar"
    objectVersion: ""
  - |
    objectPath: "/foo1"
    objectName: "bar"
    objectVersion: ""`,
				},
			},
		})).To(Succeed())

		spc := &spcv1alpha1.SecretProviderClass{}
		Expect(cli.Get(ctx, client.ObjectKey{
			Namespace: namespace.Name,
			Name:      spcName,
		}, spc)).To(Succeed(), "Failed to get secretproviderclass %#v", spc)

		e2e.Byf("%s: Installing nginx deployment", namespace.Name)

		deploymentName := "nginx-deployment"
		deploymentLabels := map[string]string{
			"app": "nginx",
		}
		replicas := new(int32)
		*replicas = 2
		readOnly := new(bool)
		*readOnly = true

		Expect(cli.Create(ctx, &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: namespace.Name,
				Labels:    deploymentLabels,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: deploymentLabels,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: deploymentLabels,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:            "container",
								Image:           "nginx",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Env: []corev1.EnvVar{
									{
										Name: "SECRET_USERNAME",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: "foosecret",
												},
												Key: "username",
											},
										},
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "secrets-store-inline",
										MountPath: "/mnt/secrets-store",
										ReadOnly:  true,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "secrets-store-inline",
								VolumeSource: corev1.VolumeSource{
									CSI: &corev1.CSIVolumeSource{
										Driver:   "secrets-store.csi.k8s.io",
										ReadOnly: readOnly,
										VolumeAttributes: map[string]string{
											"secretProviderClass": spcName,
										},
									},
								},
							},
						},
					},
				},
			},
		})).To(Succeed())

		e2e.Byf("%s: Waiting for nginx deployment is running", namespace.Name)

		deploy := &appsv1.Deployment{}
		Eventually(func() error {
			err := cli.Get(ctx, client.ObjectKey{
				Namespace: namespace.Name,
				Name:      deploymentName,
			}, deploy)
			if err != nil {
				return err
			}

			if int(deploy.Status.ReadyReplicas) != 2 {
				return errors.New("ReadyReplicas is not 2")
			}

			return nil
		}).Should(Succeed())

		e2e.Byf("%s: Reading secrets from nginx pod's volume", namespace.Name)

		pods := &corev1.PodList{}
		Expect(cli.List(ctx, pods, &client.ListOptions{
			Namespace: namespace.Name,
			LabelSelector: labels.SelectorFromValidatedSet(labels.Set(map[string]string{
				"app": "nginx",
			})),
		})).To(Succeed(), "Failed to list pods %#v", pods)

		podName := pods.Items[0].Name

		stdout, stderr, err := localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "cat", "/mnt/secrets-store/foo")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello"))

		stdout, stderr, err = localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "cat", "/mnt/secrets-store/foo1")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello1"))

		e2e.Byf("%s: Reading generated secret", namespace.Name)

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
		}).Should(Succeed())

		pwd, ok := secret.Data["pwd"]
		Expect(ok).To(BeTrue())
		Expect(string(pwd)).To(Equal("hello"))

		l, ok := secret.ObjectMeta.Labels["environment"]
		Expect(ok).To(BeTrue())
		Expect(string(l)).To(Equal("test"))

		e2e.Byf("%s: Reading environment variable of nginx pod", namespace.Name)

		stdout, stderr, err = localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "printenv", "SECRET_USERNAME")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal("hello1"))

		e2e.Byf("%s: Deleting nginx deployment", namespace.Name)

		Expect(cli.Delete(ctx, deploy)).To(Succeed())

		e2e.Byf("%s: Waiting secret is deleted", namespace.Name)

		Eventually(func() error {
			err := cli.Get(ctx, client.ObjectKey{
				Namespace: namespace.Name,
				Name:      "foosecret",
			}, secret)
			if err == nil {
				return fmt.Errorf("secret foosecret still exists")
			}

			return nil
		}).Should(Succeed())
	})

	It("test CSI inline volume with multiple secret provider class", func() {
		e2e.Byf("%s: Installing secretproviderclasses", namespace.Name)

		vaultAddress := vault.GetAddress(ctx, vault.GetAddressInput{cli, csiNamespace})

		spcValues := []struct {
			spcName          string
			secretObjectName string
		}{
			{
				spcName:          "vault-foo-sync-0",
				secretObjectName: "foosecret-0",
			},
			{
				spcName:          "vault-foo-sync-1",
				secretObjectName: "foosecret-1",
			},
		}

		for _, spcv := range spcValues {
			Expect(cli.Create(ctx, &spcv1alpha1.SecretProviderClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spcv.spcName,
					Namespace: namespace.Name,
				},
				Spec: spcv1alpha1.SecretProviderClassSpec{
					Provider: "vault",
					SecretObjects: []*spcv1alpha1.SecretObject{
						{
							SecretName: spcv.secretObjectName,
							Type:       "Opaque",
							Data: []*spcv1alpha1.SecretObjectData{
								{
									ObjectName: "foo",
									Key:        "pwd",
								},
								{
									ObjectName: "foo1",
									Key:        "username",
								},
							},
						},
					},
					Parameters: map[string]string{
						"roleName":           vault.RoleName,
						"vaultAddress":       vaultAddress,
						"vaultSkipTLSVerify": "true",
						"objects": `array:
  - |
    objectPath: "/foo"
    objectName: "bar"
    objectVersion: ""
  - |
    objectPath: "/foo1"
    objectName: "bar"
    objectVersion: ""`,
					},
				},
			})).To(Succeed())
		}

		for _, spcv := range spcValues {
			spc := &spcv1alpha1.SecretProviderClass{}
			Expect(cli.Get(ctx, client.ObjectKey{
				Namespace: namespace.Name,
				Name:      spcv.spcName,
			}, spc)).To(Succeed(), "Failed to get secretproviderclass %#v", spc)
		}

		e2e.Byf("%s: Installing nginx pod", namespace.Name)

		podName := "nginx-secrets-store-inline-multiple-crd"
		readOnly := new(bool)
		*readOnly = true

		Expect(cli.Create(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace.Name,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            "nginx",
						Image:           "nginx",
						ImagePullPolicy: corev1.PullIfNotPresent,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "secrets-store-inline-0",
								MountPath: "/mnt/secrets-store-0",
								ReadOnly:  true,
							},
							{
								Name:      "secrets-store-inline-1",
								MountPath: "/mnt/secrets-store-1",
								ReadOnly:  true,
							},
						},
						Env: []corev1.EnvVar{
							{
								Name: "SECRET_USERNAME_0",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "foosecret-0",
										},
										Key: "username",
									},
								},
							},
							{
								Name: "SECRET_USERNAME_1",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "foosecret-1",
										},
										Key: "username",
									},
								},
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "secrets-store-inline-0",
						VolumeSource: corev1.VolumeSource{
							CSI: &corev1.CSIVolumeSource{
								Driver:   "secrets-store.csi.k8s.io",
								ReadOnly: readOnly,
								VolumeAttributes: map[string]string{
									"secretProviderClass": spcValues[0].spcName,
								},
							},
						},
					},
					{
						Name: "secrets-store-inline-1",
						VolumeSource: corev1.VolumeSource{
							CSI: &corev1.CSIVolumeSource{
								Driver:   "secrets-store.csi.k8s.io",
								ReadOnly: readOnly,
								VolumeAttributes: map[string]string{
									"secretProviderClass": spcValues[1].spcName,
								},
							},
						},
					},
				},
			},
		})).To(Succeed())

		e2e.Byf("%s: Waiting for nginx pod is running", namespace.Name)

		pod := &corev1.Pod{}
		podName = "nginx-secrets-store-inline-multiple-crd"
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
		}).Should(Succeed())

		e2e.Byf("%s: Reading secret from pod", namespace.Name)

		for i, spcv := range spcValues {
			stdout, stderr, err := localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "cat", fmt.Sprintf("/mnt/secrets-store-%d/foo", i))
			Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
			Expect(strings.TrimSpace(string(stdout))).To(Equal("hello"))

			stdout, stderr, err = localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "cat", fmt.Sprintf("/mnt/secrets-store-%d/foo1", i))
			Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
			Expect(strings.TrimSpace(string(stdout))).To(Equal("hello1"))

			e2e.Byf("%s: Reading generated secret", namespace.Name)

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
			}).Should(Succeed())

			pwd, ok := secret.Data["pwd"]
			Expect(ok).To(BeTrue())
			Expect(string(pwd)).To(Equal("hello"))

			e2e.Byf("%s: Reading environment variable of nginx pod", namespace.Name)

			stdout, stderr, err = localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "printenv", fmt.Sprintf("SECRET_USERNAME_%d", i))
			Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
			Expect(strings.TrimSpace(string(stdout))).To(Equal("hello1"))
		}
	})

	AfterEach(func() {
		if input.skipCleanup {
			framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
				Deleter: clusterProxy.GetClient(),
				Name:    namespace.Name,
			})
			cancelWatches()
		}
	})
}
