# Usage

## Prerequisites

### Supported kubernetes versions

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

## Install the Secrets Store CSI Driver

**Using Helm Chart**

Follow the [guide to install driver using Helm](charts/secrets-store-csi-driver/README.md)


<details>
<summary><strong>[ALTERNATIVE DEPLOYMENT OPTION] Using Deployment Yamls</strong></summary>

```bash
kubectl apply -f deploy/rbac-secretproviderclass.yaml # update the namespace of the secrets-store-csi-driver ServiceAccount
kubectl apply -f deploy/csidriver.yaml
kubectl apply -f deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml
kubectl apply -f deploy/secrets-store.csi.x-k8s.io_secretproviderclasspodstatuses.yaml
kubectl apply -f deploy/secrets-store-csi-driver.yaml --namespace $NAMESPACE

# If using the driver to sync secrets-store content as Kubernetes Secrets, deploy the additional RBAC permissions
# required to enable this feature
kubectl apply -f deploy/rbac-secretprovidersyncing.yaml

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
csi-secrets-store-qp9r8         3/3     Running   0          4m
csi-secrets-store-zrjt2         3/3     Running   0          4m
```

You should see the following CRDs deployed:

```bash
kubectl get crd
NAME                                               
secretproviderclasses.secrets-store.csi.x-k8s.io    
```

</details>

## Use the Secrets Store CSI Driver with a Provider

Select a provider from the following list, then follow the installation steps for the provider:
-  [Azure Provider](https://github.com/Azure/secrets-store-csi-driver-provider-azure#usage)
-  [Vault Provider](https://github.com/hashicorp/secrets-store-csi-driver-provider-vault)
-  [GCP Provider](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp)

## Create your own SecretProviderClass Object

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

Here is a sample [`SecretProviderClass` custom resource](test/bats/tests/vault/vault_v1alpha1_secretproviderclass.yaml)

## Update your Deployment Yaml

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

Here is a sample [deployment yaml](test/bats/tests/vault/nginx-pod-vault-inline-volume-secretproviderclass.yaml) using the Secrets Store CSI driver.

## Secret Content is Mounted on Pod Start
On pod start and restart, the driver will call the provider binary to retrieve the secret content from the external Secrets Store you have specified in the `SecretProviderClass` custom resource. Then the content will be mounted to the container's file system. 

To validate, once the pod is started, you should see the new mounted content at the volume path specified in your deployment yaml.

```bash
kubectl exec -it nginx-secrets-store-inline ls /mnt/secrets-store/
foo
```

## [OPTIONAL] Sync with Kubernetes Secrets

In some cases, you may want to create a Kubernetes Secret to mirror the mounted content. Use the optional `secretObjects` field to define the desired state of the synced Kubernetes secret objects. **The volume mount is required for the Sync With Kubernetes Secrets** 
> NOTE: If the provider supports object alias for the mounted file, then make sure the `objectName` in `secretObjects` matches the name of the mounted content. This could be the object name or the object alias.

A `SecretProviderClass` custom resource should have the following components:
```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1alpha1
kind: SecretProviderClass
metadata:
  name: my-provider
spec:
  provider: vault                             # accepted provider options: azure or vault
  secretObjects:                              # [OPTIONAL] SecretObject defines the desired state of synced K8s secret objects
  - data:
    - key: username                           # data field to populate
      objectName: foo1                        # name of the mounted content to sync. this could be the object name or the object alias
    secretName: foosecret                     # name of the Kubernetes Secret object
    type: Opaque                              # type of the Kubernetes Secret object e.g. Opaque, kubernetes.io/tls
```
> NOTE: Here is the list of supported Kubernetes Secret types: `Opaque`, `kubernetes.io/basic-auth`, `bootstrap.kubernetes.io/token`, `kubernetes.io/dockerconfigjson`, `kubernetes.io/dockercfg`, `kubernetes.io/ssh-auth`, `kubernetes.io/service-account-token`, `kubernetes.io/tls`.  

Here is a sample [`SecretProviderClass` custom resource](test/bats/tests/vault/vault_synck8s_v1alpha1_secretproviderclass.yaml) that syncs Kubernetes secrets.

## [OPTIONAL] Set ENV VAR

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
Here is a sample [deployment yaml](test/bats/tests/vault/nginx-deployment-synck8s.yaml) that creates an ENV VAR from the synced Kubernetes secret.

## [OPTIONAL] Enable Auto Rotation of Secrets

You can setup the Secrets Store CSI Driver to periodically update the pod mount and Kubernetes Secret with the latest content from external secrets-store. Refer to [doc](docs/README.rotation.md) for steps on enabling auto rotation.

**NOTE** The CSI driver **does not restart** the application pods. It only handles updating the pod mount and Kubernetes secret similar to how Kubernetes handles updates to Kubernetes secret mounted as volumes.

