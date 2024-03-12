# Usage

## Create your own SecretProviderClass Object

To use the Secrets Store CSI driver, create a `SecretProviderClass` custom resource to provide driver configurations and provider-specific parameters to the CSI driver.

A `SecretProviderClass` custom resource should have the following components:

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: my-provider
spec:
  provider: vault                             # accepted provider options: akeyless or azure or vault or gcp
  parameters:                                 # provider-specific parameters
```

Here is a sample [`SecretProviderClass` custom resource](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/release-1.0/test/bats/tests/vault/vault_v1_secretproviderclass.yaml)

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

Here is a sample [deployment yaml](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/main/test/bats/tests/vault/pod-vault-inline-volume-secretproviderclass.yaml) using the Secrets Store CSI driver.

## Secret Content is Mounted on Pod Start

On pod start and restart, the driver will communicate with the provider using gRPC to retrieve the secret content from the external Secrets Store you have specified in the `SecretProviderClass` custom resource. Then the volume is mounted in the pod as `tmpfs` and the secret contents are written to the volume.

To validate, once the pod is started, you should see the new mounted content at the volume path specified in your deployment yaml.

```bash
kubectl exec secrets-store-inline -- ls /mnt/secrets-store/
foo
```

## [OPTIONAL] Sync with Kubernetes Secrets

Refer to [Sync as Kubernetes Secret](../topics/sync-as-kubernetes-secret.md) for steps on syncing the secrets-store content as Kubernetes secret in addition to the mount.

### [OPTIONAL] Set ENV VAR

Refer to [Set as ENV var](../topics/set-as-env-var.md) for steps on syncing the secrets-store content as Kubernetes secret and using the secret for env variables in the deployment.

## [OPTIONAL] Enable Auto Rotation of Secrets

You can setup the Secrets Store CSI Driver to periodically update the pod mount and Kubernetes Secret with the latest content from external secrets-store. Refer to [Secret Auto Rotation](../topics/secret-auto-rotation.md) for steps on enabling auto rotation.

<aside class="note warning">
<h1>NOTE</h1>

The CSI driver **does not restart** the application pods. It only handles updating the pod mount and Kubernetes secret similar to how Kubernetes handles updates to Kubernetes secret mounted as volumes.

</aside>
