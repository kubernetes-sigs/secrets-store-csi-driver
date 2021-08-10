package vault

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

// E2eVault is a fake Vault for end-to-end testing.
type E2eVault struct {
	KubeClient         *kubernetes.Clientset
	VaultNamespaceName string
}

// Vault is the interface for interacting with fake Vault.
type Vault interface {
	// GetSecret returns the value of the given secret.
	GetSecret(name string) (string, string, error)
}

// NewVault creates a new fake Vault.
func NewVault() (Vault, error) {
	kubeClient := getClientSet()
	vaultNamespaceName, err := populateVault(kubeClient)
	if err != nil {
		return nil, err
	}

	return &E2eVault{
		KubeClient:         kubeClient,
		VaultNamespaceName: vaultNamespaceName,
	}, nil
}

// GetSecret returns the value of the given secret.
func (v *E2eVault) GetSecret(name string) (string, string, error) {
	secret, err := v.KubeClient.CoreV1().Secrets(v.VaultNamespaceName).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	return string(secret.Data[name]), secret.GetLabels()["version"], nil
}

// populateVault populates the fake Vault with default secrets.
func populateVault(kubeClient *kubernetes.Clientset) (string, error) {
	vaultNamespaceName := "e2e-vault"

	// check for secret namespace
	namespaceList, err := kubeClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Error listing namespaces: %v", err)
		return vaultNamespaceName, err // don't populate if we can't list namespaces
	}

	// delete if namespace already exists
	gracePeriod := int64(0)
	for _, namespace := range namespaceList.Items {
		if namespace.Name == vaultNamespaceName {
			// delete existing vault namespace
			err := kubeClient.CoreV1().Namespaces().Delete(context.TODO(), vaultNamespaceName, metav1.DeleteOptions{
				GracePeriodSeconds: &gracePeriod,
			})
			if err != nil {
				klog.Fatalf("error deleting namespace %s: %v", vaultNamespaceName, err)
			}

			// wait for namespace to be deleted
			for {
				namespaceList, err := kubeClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
				if err != nil {
					klog.Errorf("Error listing namespaces: %v", err)
					return vaultNamespaceName, err // don't populate if we can't list namespaces
				}

				isDeleted := true
				for _, namespace := range namespaceList.Items {
					if namespace.Name == vaultNamespaceName {
						isDeleted = false
					}
				}

				if isDeleted {
					break
				}
			}
		}
	}

	// create new vault namespace
	vaultNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: vaultNamespaceName,
		},
	}
	_, err = kubeClient.CoreV1().Namespaces().Create(context.TODO(), vaultNamespace, metav1.CreateOptions{})
	if err != nil {
		klog.Fatalf("error creating namespace %s: %v", vaultNamespace, err)
	}

	// Create a foo secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
			Labels: map[string]string{
				"version": "v1",
			},
		},
		Data: map[string][]byte{
			"foo": []byte("bar"),
		},
	}
	_, err = kubeClient.CoreV1().Secrets(vaultNamespaceName).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		klog.Fatalf("error creating secret %s: %v", secret, err)
	}

	// Create a fookey secret
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fookey",
			Labels: map[string]string{
				"version": "v1",
			},
		},
		Data: map[string][]byte{
			"fookey": []byte(`-----BEGIN PUBLIC KEY-----
This is fake key
-----END PUBLIC KEY-----`),
		},
	}
	_, err = kubeClient.CoreV1().Secrets(vaultNamespaceName).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		klog.Fatalf("error creating secret %s: %v", secret, err)
	}

	return vaultNamespaceName, nil
}

// GetClientSet returns a client-go client for the cluster.
func getClientSet() *kubernetes.Clientset {
	restConfig := getRestConfig()

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Fatalf("Failed to create client-go client: %v", err)
	}

	return clientset
}

func getRestConfig() *rest.Config {
	config, err := clientcmd.LoadFromFile(clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename())
	if err == nil {
		restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			klog.Fatalf("Failed to create rest config: %v", err)
		}

		restConfig.UserAgent = "e2e-provider"
		return restConfig
	}
	klog.Warningf("Failed to load kubeconfig file: %v. Trying in-cluster config", err)

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to create in-cluster rest config: %v", err)
	}

	restConfig.UserAgent = "e2e-provider"
	return restConfig
}
