# Kubernetes Secrets Store CSI Driver

Secrets Store CSI Driver for Kubernetes secrets - Integrates secrets stores with Kubernetes via a [Container Storage Interface (CSI)](https://kubernetes-csi.github.io/docs/) volume.

The Secrets Store CSI Driver `secrets-store.csi.k8s.io` allows Kubernetes to mount multiple secrets, keys, and certs stored in enterprise-grade external secrets stores into their pods as a volume. Once the Volume is attached, the data in it is mounted into the container's file system.

## Want to help?

Join us to help define the direction and implementation of this project!

- Join the [#csi-secrets-store](https://kubernetes.slack.com/messages/csi-secrets-store) channel on [Kubernetes Slack](https://kubernetes.slack.com/).
- Join the [Mailing list](https://groups.google.com/forum/#!forum/kubernetes-secrets-store-csi-driver) to receive notifications for releases, security announcements, etc.
- Use [GitHub Issues](https://github.com/kubernetes-sigs/secrets-store-csi-driver/issues) to file bugs, request features, or ask questions asynchronously.
- Join [biweekly community meetings](https://docs.google.com/document/d/1q74nboAg0GSPcom3kLWCIoWg43Qg3mr306KNL58f2hg/edit?usp=sharing) to discuss development, issues, use cases, etc.

## Project Status

| Driver                                                                                    | Compatible Kubernetes | `secrets-store.csi.x-k8s.io` Versions |
| ----------------------------------------------------------------------------------------- | --------------------- | ------------------------------------- |
| [v1.4.0](https://github.com/kubernetes-sigs/secrets-store-csi-driver/releases/tag/v1.4.0) | 1.19+                 | `v1`, `v1alpha1 [DEPRECATED]`         |
| [v1.3.4](https://github.com/kubernetes-sigs/secrets-store-csi-driver/releases/tag/v1.3.4) | 1.19+                 | `v1`, `v1alpha1 [DEPRECATED]`         |

See
[Release Management](./release-management.md)
for additional details on versioning. We aim to release a new minor version every month and intend to support the latest
2 minor versions of the driver.

## Features

### Driver Core Functionality (Stable)

- Multiple external [secrets store providers](./providers.md)
- Pod portability with the `SecretProviderClass` `CustomResourceDefinition`
- Mounts secrets/keys/certs to pod using a CSI Inline volume
- Mount multiple secrets store objects as a single volume
- Linux and Windows containers

### Alpha Functionality

These features are not stable. If you use these be sure to consult the
[upgrade instructions](./getting-started/upgrades.md) with each upgrade.

- [Auto rotation](./topics/secret-auto-rotation.md) of mounted contents and synced Kubernetes secret
- [Sync with Kubernetes Secrets](./topics/sync-as-kubernetes-secret.md)

## Supported Providers

- [Akeyless Provider](https://github.com/akeylesslabs/akeyless-csi-provider)
- [AWS Provider](https://github.com/aws/secrets-store-csi-driver-provider-aws)
- [Azure Provider](https://azure.github.io/secrets-store-csi-driver-provider-azure/)
- [Conjur Provider](https://github.com/cyberark/conjur-k8s-csi-provider)
- [GCP Provider](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp)
- [Vault Provider](https://github.com/hashicorp/secrets-store-csi-driver-provider-vault)
