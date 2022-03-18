# Known Limitations

This document highlights the current limitations when using secrets-store-csi-driver.

<!-- toc -->

## Mounted content and Kubernetes Secret not updated

- When the secret/key is updated in external secrets store after the initial pod deployment, the updated secret is not automatically reflected in the pod mount or the Kubernetes secret.
- When the `SecretProviderClass` is updated after the pod was initially created.
- Adding/deleting objects and updating keys in existing `secretObjects` doesn't result in update of Kubernetes secrets.

The CSI driver is invoked by kubelet only during the pod volume mount. So subsequent changes in the `SecretProviderClass` after the pod has started doesn't trigger an update to the content in volume mount or Kubernetes secret.

`Enable Secret autorotation` feature has been released in `v0.0.15+`. Refer to [doc](topics/secret-auto-rotation.md) and [design doc](https://docs.google.com/document/d/1RGT0vmeUnN71n_u5fZKsSCa2YQpGw99rfGN9RlFMgHs/edit?usp=sharing) for more details.

### How to fetch the latest content with release `v0.0.14` and earlier or without `Auto rotation` feature enabled?

1. If the `SecretProviderClass` has `secretObjects` defined, then delete the Kubernetes secret.
2. Restart the application pod.

When the pod is recreated, `kubelet` invokes the CSI driver for mounting the volume. As part of this mount request, the latest content will be fetched from external secrets store and populated in the pod. The same content is then mirrored in the Kubernetes secret data.

## Secrets not rotated when using subPath volume mount

A container using `subPath` volume mount will not receive secret updates when it is rotated.

```yaml
    volumeMounts:
    - mountPath: /app/spapi/settings.ini
      name: app-config
      subPath: settings.ini
...
  volumes:
  - csi:
      driver: secrets-store.csi.k8s.io
      readOnly: true
      volumeAttributes:
        secretProviderClass: app-config
    name: app-config
```

Secrets Store CSI Driver uses [atomic writer](https://github.com/kubernetes/kubernetes/blob/master/pkg/volume/util/atomic_writer.go) to write the secret files. This is the same writer used by Kubernetes to write secret, configmap and downward API volumes. Atomic writer relies on symlinks to update the content of the file. The secret file is bind mounted into the container and is a symlink to the actual secret file in a timestamped directory. When the secret gets updated, the symlink is updated but the actual secret file bind mounted into the container remains unchanged. Refer to [kubernetes/kubernetes#50345](https://github.com/kubernetes/kubernetes/issues/50345) for more details.

The only way to get the latest content is to restart the pod or not use `subPath` volume mount.
