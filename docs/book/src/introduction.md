# Kubernetes Secrets Store CSI Driver

Secrets Store CSI driver for Kubernetes secrets - Integrates secrets stores with Kubernetes via a [Container Storage Interface (CSI)](https://kubernetes-csi.github.io/docs/) volume.

The Secrets Store CSI driver `secrets-store.csi.k8s.io` allows Kubernetes to mount multiple secrets, keys, and certs stored in enterprise-grade external secrets stores into their pods as a volume. Once the Volume is attached, the data in it is mounted into the container's file system.

## Want to help?

Join us to help define the direction and implementation of this project!

- Join the [#csi-secrets-store](https://kubernetes.slack.com/messages/csi-secrets-store) channel on [Kubernetes Slack](https://kubernetes.slack.com/).
- Join the [Mailing list](https://groups.google.com/forum/#!forum/kubernetes-secrets-store-csi-driver) to receive notifications for releases, security announcements, etc.
- Use [GitHub Issues](https://github.com/kubernetes-sigs/secrets-store-csi-driver/issues) to file bugs, request features, or ask questions asynchronously.
- Join [biweekly community meetings](https://docs.google.com/document/d/1q74nboAg0GSPcom3kLWCIoWg43Qg3mr306KNL58f2hg/edit?usp=sharing) to discuss development, issues, use cases, etc.

## Features

- Mounts secrets/keys/certs to pod using a CSI volume
- Supports mounting multiple secrets store objects as a single volume
- Supports multiple secrets stores as providers. Multiple providers can run in the same cluster simultaneously
- Supports pod portability with the SecretProviderClass CRD
- Supports windows containers (Kubernetes version v1.18+)
- Supports sync with Kubernetes Secrets (Secrets Store CSI Driver v0.0.10+)
- Support auto rotation of mounted contents and synced Kubernetes secret (Secrets Store CSI Driver v0.0.15+)

## Supported Providers

- [Azure Provider](https://azure.github.io/secrets-store-csi-driver-provider-azure/)
- [Vault Provider](https://github.com/hashicorp/secrets-store-csi-driver-provider-vault)
- [GCP Provider](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp)
