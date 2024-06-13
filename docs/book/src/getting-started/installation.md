# Installation

## Install the Secrets Store CSI Driver

### Prerequisites

#### Supported kubernetes versions

Secrets Store CSI Driver will maintain support for all actively supported Kubernetes minor releases per [Kubernetes Supported Versions policy](https://kubernetes.io/releases/version-skew-policy/). Check out the [Kubernetes releases](https://kubernetes.io/releases/) page for the latest supported Kubernetes releases.

### Deployment using Helm

Secrets Store CSI Driver allows users to customize their installation via Helm.

> Recommended to use Helm3

```bash
helm repo add secrets-store-csi-driver https://kubernetes-sigs.github.io/secrets-store-csi-driver/charts
helm install csi-secrets-store secrets-store-csi-driver/secrets-store-csi-driver --namespace kube-system
```

Running the above `helm install` command will install the Secrets Store CSI Driver on Linux nodes in the `kube-system`
namespace.

#### Optional Values

Notably the following feature must be explicitly enabled:

| Feature                                                             | Helm Parameter              |
| ------------------------------------------------------------------- | --------------------------- |
| [Sync as Kubernetes secret](../topics/sync-as-kubernetes-secret.md) | `syncSecret.enabled=true`   |
| [Secret Auto rotation](../topics/secret-auto-rotation.md)           | `enableSecretRotation=true` |

For a list of customizable values that can be injected when invoking helm install, please see the [Helm chart configurations](https://github.com/kubernetes-sigs/secrets-store-csi-driver/tree/main/charts/secrets-store-csi-driver#configuration).

### [Alternatively] Deployment using yamls

```bash
kubectl apply -f deploy/rbac-secretproviderclass.yaml
kubectl apply -f deploy/csidriver.yaml
kubectl apply -f deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml
kubectl apply -f deploy/secrets-store.csi.x-k8s.io_secretproviderclasspodstatuses.yaml
kubectl apply -f deploy/secrets-store-csi-driver.yaml

# If using the driver to sync secrets-store content as Kubernetes Secrets, deploy the additional RBAC permissions
# required to enable this feature
kubectl apply -f deploy/rbac-secretprovidersyncing.yaml

# If using the secret rotation feature, deploy the additional RBAC permissions
# required to enable this feature
kubectl apply -f deploy/rbac-secretproviderrotation.yaml

# If using the CSI Driver token requests feature (https://kubernetes-csi.github.io/docs/token-requests.html) to use
# pod/workload identity to request a token and use with providers
kubectl apply -f deploy/rbac-secretprovidertokenrequest.yaml

# [OPTIONAL] To deploy driver on windows nodes
kubectl apply -f deploy/secrets-store-csi-driver-windows.yaml
```

To validate the installer is running as expected, run the following commands:

```bash
kubectl get po --namespace=kube-system
```

You should see the Secrets Store CSI driver pods running on each agent node:

```bash
csi-secrets-store-qp9r8         3/3     Running   0          4m
csi-secrets-store-zrjt2         3/3     Running   0          4m
```

You should see the following CRDs deployed:

```bash
kubectl get crd
NAME                                               
secretproviderclasses.secrets-store.csi.x-k8s.io
secretproviderclasspodstatuses.secrets-store.csi.x-k8s.io
```

## Install External Secret Providers

Now that the Secrets Store CSI Driver has been deployed, select a provider from the supported provider list, then follow the installation steps for the provider:

- [Akeyless Provider](https://github.com/akeylesslabs/akeyless-csi-provider)
- [AWS Provider](https://github.com/aws/secrets-store-csi-driver-provider-aws)
- [Azure Provider](https://azure.github.io/secrets-store-csi-driver-provider-azure/)
- [Conjur Provider](https://github.com/cyberark/conjur-k8s-csi-provider)
- [GCP Provider](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp)
- [Vault Provider](https://github.com/hashicorp/secrets-store-csi-driver-provider-vault)
