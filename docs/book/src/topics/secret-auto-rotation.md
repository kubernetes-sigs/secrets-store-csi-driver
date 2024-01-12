# Auto rotation of mounted contents and synced Kubernetes Secrets

- **Design doc:** [Rotation Design](https://docs.google.com/document/d/1RGT0vmeUnN71n_u5fZKsSCa2YQpGw99rfGN9RlFMgHs/edit?usp=sharing)
- **Feature State:** Secrets Store CSI Driver v0.0.15 [**alpha**]

When the secret/key is updated in external secrets store after the initial pod deployment, the updated secret will be periodically updated in the pod mount and the Kubernetes Secret.

Depending on how the application consumes the secret data:

1. **Mount Kubernetes secret as a volume:** Use auto rotation feature + Sync K8s secrets feature in Secrets Store CSI Driver, application will need to watch for changes from the mounted Kubernetes Secret volume. When the Kubernetes Secret is updated by the CSI Driver, the corresponding volume contents are automatically updated.
2. **Application reads the data from container’s filesystem:** Use rotation feature in Secrets Store CSI Driver, application will need to watch for the file change from the volume mounted by the CSI driver.
3. **Using Kubernetes secret for environment variable:** The pod needs to be restarted to get the latest secret as environment variable.
   1. Use something like [Reloader](https://github.com/stakater/Reloader) to watch for changes on the synced Kubernetes secret and do rolling upgrades on pods

## Enable auto rotation

> NOTE: This alpha feature is not enabled by default.

To enable auto rotation, enable the `--enable-secret-rotation` feature gate for the `secrets-store` container in the Secrets Store CSI Driver pods. The rotation poll interval can be configured using `--rotation-poll-interval`. The default rotation poll interval is `2m`. If using helm to install the driver, set `enableSecretRotation: true` and configure the rotation poll interval by setting `rotationPollInterval`. The rotation poll interval can be tuned based on how frequently the mounted contents for all pods and Kubernetes secrets need to be resynced to the latest.

- The Secrets Store CSI Driver will update the pod mount and the Kubernetes Secret defined in `secretObjects` of SecretProviderClass periodically based on the rotation poll interval to the latest value.
- If the `SecretProviderClass` is updated after the pod was initially created
  - Adding/deleting objects and updating keys in existing `secretObjects` - the pod mount and Kubernetes secret will be updated with the new objects added to the `SecretProviderClass`.
  - Adding new `secretObject` to the existing `secretObjects` - the Kubernetes secret will be created by the controller.

## How to view the current secret versions loaded in pod mount

The Secrets Store CSI Driver creates a custom resource `SecretProviderClassPodStatus` to track the binding between a pod and `SecretProviderClass`. This `SecretProviderClassPodStatus` status also contains the details about the secrets and versions currently loaded in the pod mount.

The `SecretProviderClassPodStatus` is created in the same namespace as the pod with the name `<pod name>-<namespace>-<secretproviderclass name>`

```yaml
➜ kubectl get secretproviderclasspodstatus nginx-secrets-store-inline-crd-default-azure-spc -o yaml
...
status:
  mounted: true
  objects:
  - id: secret/secret1
    version: b82206cb5ac249918008b0b97fd1fd66
  - id: key/key1
    version: 7cc095105411491b84fe1b92ebbcf01a
  podName: nginx-secrets-store-inline-multiple-crd
  secretProviderClassName: azure-spc
  targetPath: /var/lib/kubelet/pods/1b7b0740-62d5-4776-a0df-90d060ef35ba/volumes/kubernetes.io~csi/secrets-store-inline-0/mount
```

## Limitations

The auto rotation feature is only supported with providers that have implemented gRPC server for enabling driver-provider communication.
