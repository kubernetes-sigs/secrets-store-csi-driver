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
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/azure"
	localexec "sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework/exec"
)

// AzureSpecInput is the input for AzureSpec.
type AzureSpecInput struct {
	clusterProxy framework.ClusterProxy
	skipCleanup  bool
	chartPath    string
	clientID     string
	clientSecret string
}

// AzureSpec implements a spec that testing Azure provider
func AzureSpec(ctx context.Context, inputGetter func() AzureSpecInput) {
	var (
		specName         = "azure-provider"
		input            AzureSpecInput
		namespace        *corev1.Namespace
		cancelWatches    context.CancelFunc
		cli              client.Client
		secretName       = "secret1"
		secretVersion    = ""
		secretValue      = "test"
		keyVaultName     = "csi-secrets-store-e2e"
		keyName          = "key1"
		keyVersion       = "7cc095105411491b84fe1b92ebbcf01a"
		catCommand       = "cat"
		keyValueContains = "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUlJQklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUF4K2FadlhJN2FldG5DbzI3akVScgpheklaQ2QxUlBCQVZuQU1XcDhqY05TQk5MOXVuOVJrenJHOFd1SFBXUXNqQTA2RXRIOFNSNWtTNlQvaGQwMFNRCk1aODBMTlNxYkkwTzBMcWMzMHNLUjhTQ0R1cEt5dkpkb01LSVlNWHQzUlk5R2Ywam1ucHNKOE9WbDFvZlRjOTIKd1RINXYyT2I1QjZaMFd3d25MWlNiRkFnSE1uTHJtdEtwZTVNcnRGU21nZS9SL0J5ZXNscGU0M1FubnpndzhRTwpzU3ZMNnhDU21XVW9WQURLL1MxREU0NzZBREM2a2hGTjF5ZHUzbjVBcnREVGI0c0FjUHdTeXB3WGdNM3Y5WHpnClFKSkRGT0JJOXhSTW9UM2FjUWl0Z0c2RGZibUgzOWQ3VU83M0o3dUFQWUpURG1pZGhrK0ZFOG9lbjZWUG9YRy8KNXdJREFRQUIKLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0t"
		labelValue       = "test"
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

		spcName := "azure"
		Expect(cli.Create(ctx, &spcv1alpha1.SecretProviderClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      spcName,
				Namespace: namespace.Name,
			},
			Spec: spcv1alpha1.SecretProviderClassSpec{
				Provider: "azure",
				Parameters: map[string]string{
					"usePodIdentity": "false",
					"keyvaultName":   keyVaultName,
					"objects": `array:
  - |
    objectName: ` + secretName + `
    objectType: "secret"
    objectVersion: ` + secretVersion + `
  - |
    objectName: ` + keyName + `
    objectType: "key"
	objectVersion: ` + keyVersion,
					"tenantId": "$TENANT_ID",
				},
			},
		})).To(Succeed())

		spc := &spcv1alpha1.SecretProviderClass{}
		Expect(cli.Get(ctx, client.ObjectKey{
			Namespace: namespace.Name,
			Name:      spcName,
		}, spc)).To(Succeed(), "Failed to get secretproviderclass %#v", spc)

		e2e.Byf("%s: Installing secrets-store-creds", namespace.Name)

		azure.InstallSecretsStoreCredential(ctx, azure.InstallSecretsStoreCredentialInput{
			Creator:      cli,
			Namespace:    namespace.Name,
			ClientID:     input.clientID,
			ClientSecret: input.clientSecret,
		})

		e2e.Byf("%s: Installing nginx pod", namespace.Name)

		podName := "nginx-secrets-store-inline-crd"
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
								NodePublishSecretRef: &corev1.LocalObjectReference{
									Name: azure.SecretsStoreCreds,
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

		e2e.Byf("%s: Reading azure kv secret from pod", namespace.Name)

		stdout, stderr, err := localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, catCommand, "/mnt/secrets-store/"+secretName)
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal(secretValue))

		e2e.Byf("%s: Reading azure kv key from pod", namespace.Name)

		stdout, stderr, err = localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, catCommand, "/mnt/secrets-store/"+keyName)
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		encoded := base64.StdEncoding.EncodeToString(bytes.TrimSpace(stdout))
		Expect(err).To(Succeed())
		Expect(encoded).To(Equal(keyValueContains))
	})

	It("test Sync with K8s secrets", func() {
		e2e.Byf("%s: Installing secretproviderclass", namespace.Name)

		spcName := "azure-sync"
		Expect(cli.Create(ctx, &spcv1alpha1.SecretProviderClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      spcName,
				Namespace: namespace.Name,
			},
			Spec: spcv1alpha1.SecretProviderClassSpec{
				Provider: "azure",
				SecretObjects: []*spcv1alpha1.SecretObject{
					{
						SecretName: "foosecret",
						Labels: map[string]string{
							"environment": "test",
						},
						Data: []*spcv1alpha1.SecretObjectData{
							{
								ObjectName: "secretalias",
								Key:        "username",
							},
						},
					},
				},
				Parameters: map[string]string{
					"usePodIdentity": "false",
					"keyvaultName":   keyVaultName,
					"objects": `array:
  - |
    objectName: ` + secretName + `
    objectType: "secret"
    objectAlias: "secretalias"
    objectVersion: ` + secretVersion + `
  - |
    objectName: ` + keyName + `
    objectType: "key"
	objectVersion: ` + keyVersion,
					"tenantId": "$TENANT_ID",
				},
			},
		})).To(Succeed())

		spc := &spcv1alpha1.SecretProviderClass{}
		Expect(cli.Get(ctx, client.ObjectKey{
			Namespace: namespace.Name,
			Name:      spcName,
		}, spc)).To(Succeed(), "Failed to get secretproviderclass %#v", spc)

		e2e.Byf("%s: Installing secrets-store-creds", namespace.Name)

		azure.InstallSecretsStoreCredential(ctx, azure.InstallSecretsStoreCredentialInput{
			Creator:      cli,
			Namespace:    namespace.Name,
			ClientID:     input.clientID,
			ClientSecret: input.clientSecret,
		})

		e2e.Byf("%s: Installing nginx deployment", namespace.Name)

		deploymentName := "nginx-deployment"
		Expect(cli.Create(ctx, nginxDeploymentSyncK8sAzure(deploymentName, namespace.Name, spcName))).To(Succeed())

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

		e2e.Byf("%s: Reading secret from pod", namespace.Name)

		pods := &corev1.PodList{}
		Expect(cli.List(ctx, pods, &client.ListOptions{
			Namespace: namespace.Name,
			LabelSelector: labels.SelectorFromValidatedSet(labels.Set(map[string]string{
				"app": "nginx",
			})),
		})).To(Succeed(), "Failed to list pods %#v", pods)

		podName := pods.Items[0].Name

		stdout, stderr, err := localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, catCommand, "/mnt/secrets-store/secretalias")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal(secretValue))

		stdout, stderr, err = localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, catCommand, "/mnt/secrets-store/"+keyName)
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		encoded := base64.StdEncoding.EncodeToString(bytes.TrimSpace(stdout))
		Expect(encoded).To(Equal(keyValueContains))

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

		pwd, ok := secret.Data["username"]
		Expect(ok).To(BeTrue())
		Expect(string(pwd)).To(Equal(secretValue))

		l, ok := secret.ObjectMeta.Labels["environment"]
		Expect(ok).To(BeTrue())
		Expect(string(l)).To(Equal(labelValue))

		e2e.Byf("%s: Reading environment variable of nginx pod", namespace.Name)

		stdout, stderr, err = localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "printenv", "SECRET_USERNAME")
		Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(strings.TrimSpace(string(stdout))).To(Equal(secretValue))

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

	It("test CSI inline volume should fail when no secret provider class in same namespace", func() {
		e2e.Byf("%s: Installing nginx deployment", namespace.Name)

		deploymentName := "nginx-deployment"
		spcName := "azure-sync"
		Expect(cli.Create(ctx, nginxDeploymentSyncK8sAzure(deploymentName, namespace.Name, spcName))).To(Succeed())

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

			if int(deploy.Status.ReadyReplicas) != 0 {
				return errors.New("ReadyReplicas is not 0")
			}

			return nil
		}).Should(Succeed())

		// TODO: Validate event 'FailedMount.*failed to get secretproviderclass negative-test-ns/azure-sync.*not found'"
	})

	It("test CSI inline volume with multiple secret provider class", func() {
		e2e.Byf("%s: Installing secretproviderclasses", namespace.Name)

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
			Expect(cli.Create(ctx, &spcv1alpha1.SecretProviderClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spcv.spcName,
					Namespace: namespace.Name,
				},
				Spec: spcv1alpha1.SecretProviderClassSpec{
					Provider: "azure",
					SecretObjects: []*spcv1alpha1.SecretObject{
						{
							SecretName: spcv.secretObjectName,
							Type:       "Opaque",
							Data: []*spcv1alpha1.SecretObjectData{
								{
									ObjectName: "secretalias",
									Key:        "username",
								},
							},
						},
					},
					Parameters: map[string]string{
						"usePodIdentity": "false",
						"keyvaultName":   keyVaultName,
						"objects": `array:
  - |
    objectName: ` + secretName + `
    objectType: "secret"
    objectVersion: ` + secretVersion + `
    objectAlias: "secretalias"
  - |
    objectName: ` + keyName + `
    objectType: "key"
	objectVersion: ` + keyVersion,
						"tenantId": "$TENANT_ID",
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
								NodePublishSecretRef: &corev1.LocalObjectReference{
									Name: azure.SecretsStoreCreds,
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
								NodePublishSecretRef: &corev1.LocalObjectReference{
									Name: azure.SecretsStoreCreds,
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

		e2e.Byf("%s: Reading secret from pod", namespace.Name)

		for i, spcv := range spcValues {
			stdout, stderr, err := localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, catCommand, fmt.Sprintf("/mnt/secrets-store-%d/secretalias", i))
			Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
			Expect(strings.TrimSpace(string(stdout))).To(Equal(secretValue))

			stdout, stderr, err = localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, catCommand, fmt.Sprintf("/mnt/secrets-store-%d/%s", i, keyName))
			Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
			encoded := base64.StdEncoding.EncodeToString(bytes.TrimSpace(stdout))
			Expect(encoded).To(Equal(keyValueContains))

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

			pwd, ok := secret.Data["username"]
			Expect(ok).To(BeTrue())
			Expect(string(pwd)).To(Equal(secretValue))

			l, ok := secret.ObjectMeta.Labels["environment"]
			Expect(ok).To(BeTrue())
			Expect(string(l)).To(Equal(labelValue))

			e2e.Byf("%s: Reading environment variable of nginx pod", namespace.Name)

			stdout, stderr, err = localexec.KubectlExec(input.clusterProxy.GetKubeconfigPath(), podName, namespace.Name, "printenv", fmt.Sprintf("SECRET_USERNAME_%d", i))
			Expect(err).To(Succeed(), "stdout=%s, stderr=%s", stdout, stderr)
			Expect(strings.TrimSpace(string(stdout))).To(Equal(secretValue))
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

func nginxDeploymentSyncK8sAzure(name, namespace, spcName string) *appsv1.Deployment {
	deploymentLabels := map[string]string{
		"app": "nginx",
	}
	replicas := new(int32)
	*replicas = 2
	readOnly := new(bool)
	*readOnly = true

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
									NodePublishSecretRef: &corev1.LocalObjectReference{
										Name: azure.SecretsStoreCreds,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
