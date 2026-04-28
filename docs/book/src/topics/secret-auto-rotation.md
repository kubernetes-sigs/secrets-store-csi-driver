# Auto rotation of mounted contents and synced Kubernetes Secrets

- **Design doc:** [Rotation Design](https://docs.google.com/document/d/1RGT0vmeUnN71n_u5fZKsSCa2YQpGw99rfGN9RlFMgHs/edit?usp=sharing)
- **Feature State:** Secrets Store CSI Driver v0.0.15 [**alpha**]

When the secret/key is updated in external secrets store after the initial pod deployment, the updated secret will be periodically updated in the pod mount and the Kubernetes Secret.

Depending on how the application consumes the secret data:

1. **Mount Kubernetes secret as a volume:** Use auto rotation feature + Sync K8s secrets feature in Secrets Store CSI Driver, application will need to watch for changes from the mounted Kubernetes Secret volume. When the Kubernetes Secret is updated by the CSI Driver, the corresponding volume contents are automatically updated.
2. **Application reads the data from container’s filesystem:** Use rotation feature in Secrets Store CSI Driver, application will need to watch for the file change from the volume mounted by the CSI driver.
3. **Using Kubernetes secret for environment variable:** The pod needs to be restarted to get the latest secret as environment variable.
   1. Use something like [Reloader](https://github.com/stakater/Reloader) to watch for changes on the synced Kubernetes secret and do rolling upgrades on pods

## How rotation works

Starting in v1.6.0, secret rotation uses the CSI [`RequiresRepublish`](https://kubernetes-csi.github.io/docs/ephemeral-local-volumes.html) mechanism. The CSIDriver object sets `requiresRepublish: true`, which causes kubelet to periodically call `NodePublishVolume` for all pods using the driver. When `--enable-secret-rotation=true` is set, the driver re-fetches secrets from the provider during these calls.

> **Note:** Setting `requiresRepublish: true` on the CSIDriver does **not** enable rotation by default. The driver ignores republish calls for already-mounted volumes unless `--enable-secret-rotation=true` is set. Users who don’t use rotation will see no behavior change.

This approach removes the need for the previously required privileged RBAC permissions (listing pods, secrets, and creating service account tokens). The dedicated rotation controller and its associated RBAC resources have been removed.

## Enable auto rotation

> NOTE: This alpha feature is not enabled by default.

To enable auto rotation, set the `--enable-secret-rotation` flag to `true` for the `secrets-store` container in the Secrets Store CSI Driver pods. The `--rotation-poll-interval` flag (default `2m`) controls the minimum cache duration between rotations — if kubelet triggers a republish call before this interval has elapsed since the last update, the driver skips the rotation. Rotation happens on the first republish call *after* this duration expires, so exact timing depends on kubelet’s republish cadence. If using Helm to install the driver, set `enableSecretRotation: true` and configure the cache duration by setting `rotationPollInterval`.

- The Secrets Store CSI Driver will update the pod mount and the Kubernetes Secret defined in `secretObjects` of SecretProviderClass when secrets are re-fetched during rotation.
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
