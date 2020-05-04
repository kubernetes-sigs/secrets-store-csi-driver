# Kubernetes-Secrets-Store-CSI-Driver

[![Build status](https://prow.k8s.io/badge.svg?jobs=secrets-store-csi-driver-e2e-vault-postsubmit)](https://testgrid.k8s.io/sig-auth-secrets-store-csi-driver#secrets-store-csi-driver-e2e-vault-postsubmit)

Secrets Store CSI driver for Kubernetes secrets - Integrates secrets stores with Kubernetes via a [Container Storage Interface (CSI)](https://kubernetes-csi.github.io/docs/) volume.

The Secrets Store CSI driver `secrets-store.csi.k8s.io` allows Kubernetes to mount multiple secrets, keys, and certs stored in enterprise-grade external secrets stores into their pods as a volume. Once the Volume is attached, the data in it is mounted into the container's file system.

## Features

- Mounts secrets/keys/certs to pod using a CSI volume
- Supports CSI Inline volume (Kubernetes version v1.15+)
- Supports mounting multiple secrets store objects as a single volume
- Supports pod identity to restrict access with specific identities (Azure provider only)
- Supports multiple secrets stores as providers. Multiple providers can run in the same cluster simultaneously.
- Supports pod portability with the SecretProviderClass CRD
- Supports windows containers (Kubernetes version v1.18+)
- Supports sync with Kubernetes Secrets (Secrets Store CSI Driver v0.0.10+)

#### Table of Contents

- [How It Works](#how-it-works)
- [Demo](#demo)
- [Usage](#usage)
- [Providers](#providers)
  - [Azure Key Vault Provider](https://github.com/Azure/secrets-store-csi-driver-provider-azure) - Supports Linux and Windows
  - [HashiCorp Vault Provider](https://github.com/hashicorp/secrets-store-csi-driver-provider-vault) - Supports Linux
  - [Adding a New Provider via the Provider Interface](#criteria-for-supported-providers)
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

Recommended Kubernetes version: v1.16.0+

> NOTE: The CSI Inline Volume feature was introduced in Kubernetes v1.15.x. Version 1.15.x will require the `CSIInlineVolume` feature gate to be updated in the cluster. Version 1.16+ does not require any feature gate.

<details>
<summary><strong> For v1.15.x, update CSI Inline Volume feature gate </strong></summary>

The CSI Inline Volume feature was introduced in Kubernetes v1.15.x. We need to make the following updates to include the `CSIInlineVolume` feature gate:

- Update the API Server manifest to append the following feature gate:

```yaml
--feature-gates=CSIInlineVolume=true
```

- Update Kubelet manifest on each node to append the `CSIInlineVolume` feature gate:

```yaml
--feature-gates=CSIInlineVolume=true
```
</details>

### Install the Secrets Store CSI Driver

**Using Helm Chart**

Follow the [guide to install driver using Helm](charts/secrets-store-csi-driver/README.md)


<details>
<summary><strong>[ALTERNATIVE DEPLOYMENT OPTION] Using Deployment Yamls</strong></summary>

```bash
kubectl apply -f deploy/rbac-secretproviderclass.yaml # update the namespace of the secrets-store-csi-driver ServiceAccount
kubectl apply -f deploy/csidriver.yaml
kubectl apply -f deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml
kubectl apply -f deploy/secrets-store-csi-driver.yaml --namespace $NAMESPACE

# [OPTIONAL] For kubernetes version < 1.16 running `kubectl apply -f deploy/csidriver.yaml` will fail. To install the driver run
kubectl apply -f deploy/csidriver-1.15.yaml

# [OPTIONAL] To deploy driver on windows nodes
kubectl apply -f deploy/secrets-store-csi-driver-windows.yaml --namespace $NAMESPACE
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

### Use the Secrets Store CSI Driver with a Provider

Select a provider from the following list, then follow the installation steps for the provider:
-  [Azure Provider](https://github.com/Azure/secrets-store-csi-driver-provider-azure#usage)
-  [Vault Provider](https://github.com/hashicorp/secrets-store-csi-driver-provider-vault)

### Create your own SecretProviderClass Object

To use the Secrets Store CSI driver, create a `SecretProviderClass` custom resource to provide driver configurations and provider-specific parameters to the CSI driver.

A `SecretProviderClass` custom resource should have the following components:
```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1alpha1
kind: SecretProviderClass
metadata:
  name: my-provider
spec:
  provider: vault                             # accepted provider options: azure or vault
  parameters:                                 # provider-specific parameters
```

Here is a sample [`SecretProviderClass` custom resource](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/master/test/bats/tests/vault_v1alpha1_secretproviderclass.yaml)

### Update your Deployment Yaml

To ensure your application is using the Secrets Store CSI driver, update your deployment yaml to use the `secrets-store.csi.k8s.io` driver and reference the `SecretProviderClass` resource created in the previous step.

```yaml
volumes:
  - name: secrets-store-inline
    csi:
      driver: secrets-store.csi.k8s.io
      readOnly: true
      volumeAttributes:
        secretProviderClass: "my-provider"
```

Here is a sample [deployment yaml](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/master/test/bats/tests/nginx-pod-vault-inline-volume-secretproviderclass.yaml) using the Secrets Store CSI driver.

### Secret Content is Mounted on Pod Start
On pod start and restart, the driver will call the provider binary to retrieve the secret content from the external Secrets Store you have specified in the `SecretProviderClass` custom resource. Then the content will be mounted to the container's file system. 

To validate, once the pod is started, you should see the new mounted content at the volume path specified in your deployment yaml.

```bash
kubectl exec -it nginx-secrets-store-inline ls /mnt/secrets-store/
foo
```

### [OPTIONAL] Sync with Kubernetes Secrets

In some cases, you may want to create a Kubernetes Secret to mirror the mounted content. Use the optional `secretObjects` field to define the desired state of the synced Kubernetes secret objects.

A `SecretProviderClass` custom resource should have the following components:
```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1alpha1
kind: SecretProviderClass
metadata:
  name: my-provider
spec:
  provider: vault                             # accepted provider options: azure or vault
  parameters:                                 # provider-specific parameters
  secretObjects:                              # [OPTIONAL] SecretObject defines the desired state of synced K8s secret objects
  - data:
    - key: username                           # data field to populate
      objectName: foo1                        # name of the object to sync
    secretName: foosecret                     # name of the Kubernetes Secret object
    type: Opaque                              # type of the Kubernetes Secret object e.g. Opaque, kubernetes.io/tls
```
> NOTE: Here is the list of [supported Kubernetes Secret types](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/master/pkg/secrets-store/utils.go#L660-L675). 

Here is a sample [`SecretProviderClass` custom resource](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/master/test/bats/tests/vault_synck8s_v1alpha1_secretproviderclass.yaml) that syncs Kubernetes secrets.

### [OPTIONAL] Set ENV VAR

Once the secret is created, you may wish to set an ENV VAR in your deployment to reference the new Kubernetes secret.

```yaml
spec:
  containers:
  - image: nginx
    name: nginx
    env:
    - name: SECRET_USERNAME
      valueFrom:
        secretKeyRef:
          name: foosecret
          key: username
```
Here is a sample [deployment yaml](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/master/test/bats/tests/nginx-deployment-synck8s.yaml) that creates an ENV VAR from the synced Kubernetes secret.


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

End-to-end tests automatically runs on Prow when a PR is submitted. If you want to run using a local or remote Kubernetes cluster, make sure to have `kubectl`, `helm` and `bats` set up in your local environment and then run `make e2e-azure` or `make e2e-vault` with custom images.

Job config for test jobs run for each PR in prow can be found [here](https://github.com/kubernetes/test-infra/blob/master/config/jobs/kubernetes-sigs/secrets-store-csi-driver/secrets-store-csi-driver-config.yaml)

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
