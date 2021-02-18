# Kubernetes Secrets Store CSI Driver

[![Build status](https://prow.k8s.io/badge.svg?jobs=secrets-store-csi-driver-e2e-vault-postsubmit)](https://testgrid.k8s.io/sig-auth-secrets-store-csi-driver#secrets-store-csi-driver-e2e-vault-postsubmit)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/kubernetes-sigs/secrets-store-csi-driver)
[![Go Report Card](https://goreportcard.com/badge/kubernetes-sigs/secrets-store-csi-driver)](https://goreportcard.com/report/kubernetes-sigs/secrets-store-csi-driver)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/kubernetes-sigs/secrets-store-csi-driver)

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
- Supports CSI Inline volume (Kubernetes version v1.15+)
- Supports mounting multiple secrets store objects as a single volume
- Supports multiple secrets stores as providers. Multiple providers can run in the same cluster simultaneously.
- Supports pod portability with the SecretProviderClass CRD
- Supports windows containers (Kubernetes version v1.18+)
- Supports sync with Kubernetes Secrets (Secrets Store CSI Driver v0.0.10+)
- Supports multiple secrets stores providers in the same cluster.

## Demo

![Secrets Store CSI Driver Demo](/img/demo.gif "Secrets Store CSI Driver Azure Key Vault Provider Demo")

## Getting Started

Check out the [installation instructions](https://secrets-store-csi-driver.sigs.k8s.io/getting-started/installation.html) to deploy the Secrets Store CSI Driver and providers. Get familiar with our [CRDs and core components](https://secrets-store-csi-driver.sigs.k8s.io/concepts.html)

## Documentation

Please see the [docs](https://secrets-store-csi-driver.sigs.k8s.io) for more in-depth information and supported features.

## Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
