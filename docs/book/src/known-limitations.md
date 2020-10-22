# Known Limitations

This document highlights the current limitations when using secrets-store-csi-driver.

## Mounted content and Kubernetes Secret not updated after secret is updated in external secrets-store

When the secret/key is updated in external secrets store after the inital pod deployment, the updated secret is not automatically reflected in the pod mount or the Kubernetes secret. 

This feature is planned for release `v0.0.15+`. See [design doc](https://docs.google.com/document/d/1RGT0vmeUnN71n_u5fZKsSCa2YQpGw99rfGN9RlFMgHs/edit?usp=sharing) for more details.

### How to fetch the latest content with release `v0.0.14` and earlier?

1. If the `SecretProviderClass` has `secretObjects` defined, then delete the Kubernetes secret.
2. Restart the application pod.

When the pod is recreated, `kubelet` invokes the CSI driver for mounting the volume. As part of this mount request, the latest content will be fetched from external secrets store and populated in the pod. The same content is then mirrored in the Kubenetes secret data.
