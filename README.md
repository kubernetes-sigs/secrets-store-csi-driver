# Kubernetes-Secrets-Store-CSI-Driver

Secrets Store CSI driver for Kubernetes secrets - Integrates secrets stores with Kubernetes via a [Container Storage Interface (CSI)](https://kubernetes-csi.github.io/docs/) volume.

The Secrets Store CSI driver `secrets-store.csi.k8s.com` allows Kubernetes to mount multiple secrets, keys, and certs stored in enterprise-grade external secrets stores into their pods as a volume. Once the Volume is attached, the data in it is mounted into the container's file system.

[![Build Status](https://travis-ci.org/deislabs/secrets-store-csi-driver.svg?branch=master)](https://travis-ci.org/deislabs/secrets-store-csi-driver)

## Features

- Mounts secrets/keys/certs to pod using a CSI volume
- Supports CSI Inline volume (Kubernetes version v1.15+)
- Supports mounting multiple secrets store objects as a single volume
- Supports pod identity to restrict access with specific identities (Azure provider only)
- Supports multiple secrets stores as providers. Multiple providers can run in the same cluster simultaneously.
- Supports pod portability with the SecretProviderClass CRD

#### Table of Contents

- [How It Works](#how-it-works)
- [Demo](#demo)
- [Usage](#usage)
- [Providers](#providers)
  - [Azure Key Vault Provider](https://github.com/Azure/secrets-store-csi-driver-provider-azure)
  - [HashiCorp Vault Provider](https://github.com/hashicorp/secrets-store-csi-driver-provider-vault)
  - [Adding a New Provider via the Provider Interface](#adding-a-new-provider-via-the-provider-interface)
- [Testing](#testing)
  - [Unit Tests](#unit-tests)
  - [End-to-end Tests](#end-to-end-tests)
- [Known Issues and Workarounds](#known-issues-and-workarounds)
- [Contributing](#contributing)

## How It Works

The diagram below illustrates how Secrets Store CSI Volume works.

![diagram](img/diagram.png)

## Demo

![Secrets Store CSI Driver Demo](img/demo.gif "Secrets Store CSI Driver Azure Key Vault Provider Demo")

## Usage

### Prerequisites

#### Supported kubernetes versions

secrets-store-csi-driver is supported only for cluster versions v1.15.0+

#### Mount Secret Data to Resource through Inline Volume

* Deploy a Kubernetes cluster v1.15.0+ and make sure it's reachable. The CSI Inline Volume feature was introduced in v1.15.0.
* Update the API Server manifest to append the following feature gate:

```yaml
--feature-gates=CSIInlineVolume=true
```

- Update Kubelet manifest on each node to append the `CSIInlineVolume` feature gate:

```yaml
--feature-gates=CSIInlineVolume=true
```

### Install the Secrets Store CSI Driver

#### Using Helm Chart

Make sure you already have helm CLI installed. To install the secrets store csi driver:

```bash
NAMESPACE=dev
helm install charts/secrets-store-csi-driver -n csi-secrets-store --namespace $NAMESPACE
```

Expected output:

```console
NAME:   csi-secrets-store
NAMESPACE: dev
STATUS: DEPLOYED

RESOURCES:
==> v1/ClusterRole
NAME                        AGE
secretproviderclasses-role  1s

==> v1/ClusterRoleBinding
NAME                               AGE
secretproviderclasses-rolebinding  0s

==> v1/DaemonSet
NAME                                        AGE
csi-secrets-store-secrets-store-csi-driver  0s

==> v1/Pod(related)
NAME                                              AGE
csi-secrets-store-secrets-store-csi-driver-hb8gb  0s
csi-secrets-store-secrets-store-csi-driver-rk7hg  0s

==> v1/ServiceAccount
NAME                      AGE
secrets-store-csi-driver  1s

==> v1beta1/CSIDriver
NAME                       AGE
secrets-store.csi.k8s.com  0s

==> v1beta1/CustomResourceDefinition
NAME                                              AGE
secretproviderclasses.secrets-store.csi.x-k8s.io  1s


NOTES:
The Secrets Store CSI Driver is getting deployed to your cluster.

To verify that Secrets Store CSI Driver has started, run:

  kubectl --namespace=dev get pods -l "app=secrets-store-csi-driver"

Now you can follow these steps https://github.com/kubernetes-sigs/secrets-store-csi-driver#use-the-secrets-store-csi-driver
to create a SecretProviderClass resource, and a deployment using the SecretProviderClass.

```
#### Using Helm without Tiller

You can also template this chart locally without Tiller and apply the result using `kubectl`.

```bash
helm template charts/secrets-store-csi-driver --name csi-secrets-store --namespace $NAMESPACE > manifest.yml
kubectl apply -f manifest.yml
```


<details>
<summary><strong>[ALTERNATIVE DEPLOYMENT OPTION] Using Deployment Yamls</strong></summary>

```bash
kubectl apply -f deploy/rbac-secretproviderclass.yaml # update the namespace of the secrets-store-csi-driver ServiceAccount
kubectl apply -f deploy/csidriver.yaml
kubectl apply -f deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml
kubectl apply -f deploy/secrets-store-csi-driver.yaml --namespace $NAMESPACE

# [OPTIONAL] For kubernetes version < 1.16 running `kubectl apply -f deploy/csidriver.yaml` will fail. To install the driver run
kubectl apply -f deploy/csidriver-1.15.yaml
```

To validate the installer is running as expected, run the following commands:

```bash
kubectl get po --namespace $NAMESPACE
```

You should see the Secrets Store CSI driver pods running on each agent node:

```bash
csi-secrets-store-qp9r8         2/2     Running   0          4m
csi-secrets-store-zrjt2         2/2     Running   0          4m
```

You should see the following CRDs deployed:

```bash
kubectl get crd
NAME                                               
secretproviderclasses.secrets-store.csi.x-k8s.io    
```

</details>

### Use the Secrets Store CSI Driver

1. Select a provider from the [list of supported providers](#providers) and deploy the provider yaml

```bash
# [REQUIRED FOR AZURE PROVIDER]
kubectl apply -f https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/deployment/provider-azure-installer.yaml --namespace $NAMESPACE

# [REQUIRED FOR VAULT PROVIDER]
kubectl apply -f https://raw.githubusercontent.com/hashicorp/secrets-store-csi-driver-provider-vault/master/deployment/provider-vault-installer.yaml --namespace $NAMESPACE

```

You should see the following pods deployed for the provider(s) you selected. For example, for the Azure Key Vault provider:

```bash
csi-secrets-store-provider-azure-pksfd             2/2     Running   0          4m
csi-secrets-store-provider-azure-sxht2             2/2     Running   0          4m
```

1. Create a `secretproviderclasses` resource to provide provider-specific parameters for the Secrets Store CSI driver. Follow [specific deployment steps](#providers) for the selected provider to update all required fields [see example secretproviderclass](pkg/providers/azure/examples/v1alpha1_secretproviderclass.yaml).

      ```yaml
      apiVersion: secrets-store.csi.x-k8s.io/v1alpha1
      kind: SecretProviderClass
      metadata:
        name: azure-kvname
      spec:
        provider: azure                   # accepted provider options: azure or vault
        parameters:
          usePodIdentity: "false"         # [OPTIONAL for Azure] if not provided, will default to "false"
          keyvaultName: "kvname"          # the name of the KeyVault
          objects:  |
            array:
              - |
                objectName: secret1
                objectType: secret        # object types: secret, key or cert
                objectVersion: ""         # [OPTIONAL] object versions, default to latest if empty
              - |
                objectName: key1
                objectType: key
                objectVersion: ""
          resourceGroup: "rg1"            # the resource group of the KeyVault
          subscriptionId: "subid"         # the subscription ID of the KeyVault
          tenantId: "tid"                 # the tenant ID of the KeyVault

      ```
1. Update your [deployment yaml](pkg/providers/azure/examples/nginx-pod-secrets-store-inline-volume-secretproviderclass.yaml) to use the Secrets Store CSI driver and reference the `secretProviderClass` resource created in the previous step

    ```yaml
    volumes:
      - name: secrets-store-inline
        csi:
          driver: secrets-store.csi.k8s.com
          readOnly: true
          volumeAttributes:
            secretProviderClass: "azure-kvname"
    ```

1. Deploy your resource with the inline CSI volume using the Secrets Store CSI driver

    ```bash
    kubectl apply -f pkg/providers/azure/examples/nginx-pod-secrets-store-inline-volume-secretproviderclass.yaml
    ```

1. Validate the pod has access to the secret from your secrets store instance:

    ```bash
    kubectl exec -it nginx-secrets-store-inline ls /mnt/secrets-store/
    secret1
    ```

## Providers

This project features a pluggable provider interface developers can implement that defines the actions of the Secrets Store CSI driver. This enables retrieval of sensitive objects stored in an enterprise-grade external secrets store into Kubernetes while continue to manage these objects outside of Kubernetes.

### Criteria for Supported Providers

Here is a list of criteria for supported provider:
1. Code audit of the provider implementation to ensure it adheres to the required provider-driver interface, which includes:
    - implementation of provider command args https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/master/pkg/secrets-store/nodeserver.go#L223-L236
    - provider binary naming convention and semver convention
    - provider binary deployment volume path
    - provider logs are written to stdout and stderr so they can be part of the driver logs
1. Add provider to the e2e test suite to demonstrate it functions as expected https://github.com/kubernetes-sigs/secrets-store-csi-driver/tree/master/test/bats Please use existing providers e2e tests as a reference.
1. If any update is made by a provider (not limited to security updates), the provider is expected to update the provider's e2e test in this repo

### Removal from Supported Providers

Failure to adhere to the [Criteria for Supported Providers](#criteria-for-supported-providers) will result in the removal of the provider from the supported list and subject to another review before it can be added back to the list of supported providers.

When a provider's e2e tests are consistently failing with the latest version of the driver, the driver maintainers will coordinate with the provider maintainers to provide a fix. If the test failures are not resolved within 4 weeks, then the provider will be removed from the list of supported providers. 

## Testing

### Unit Tests

Run unit tests locally with `make test`.

### End-to-end Tests

End-to-end tests automatically runs on Travis CI when a PR is submitted. If you want to run using a local or remote Kubernetes cluster, make sure to have `kubectl`, `helm` (with `tiller` running on the cluster) and `bats` set up in your local environment and then run `make e2e`. You can find the steps in `.travis.yml` for getting started for setting up your environment, which uses Kind to set up a cluster.

## Known Issues and Workarounds


## Troubleshooting

- To troubleshoot issues with the csi driver, you can look at logs from the `secrets-store` container of the csi driver pod running on the same node as your application pod:
  ```bash
  kubectl get pod -o wide
  # find the secrets store csi driver pod running on the same node as your application pod

  kubectl logs csi-secrets-store-secrets-store-csi-driver-7x44t secrets-store
  ```

## Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
