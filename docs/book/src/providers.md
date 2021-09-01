# Providers

This project features a pluggable provider interface developers can implement that defines the actions of the Secrets Store CSI driver. This enables retrieval of sensitive objects stored in an enterprise-grade external secrets store into Kubernetes while continue to manage these objects outside of Kubernetes.

## Criteria for Supported Providers

Here is a list of criteria for supported provider:

1. Code audit of the provider implementation to ensure it adheres to the required provider-driver interface - [Implementing a Provider for Secrets Store CSI Driver](#implementing-a-provider-for-secrets-store-csi-driver)
2. Add provider to the [e2e test suite](https://github.com/kubernetes-sigs/secrets-store-csi-driver/tree/master/test/bats) to demonstrate it functions as expected. Please use existing providers e2e tests as a reference.
3. If any update is made by a provider (not limited to security updates), the provider is expected to update the provider's e2e test in this repo.

## Removal from Supported Providers

Failure to adhere to the [Criteria for Supported Providers](#criteria-for-supported-providers) will result in the removal of the provider from the supported list and subject to another review before it can be added back to the list of supported providers.

When a provider's e2e tests are consistently failing with the latest version of the driver, the driver maintainers will coordinate with the provider maintainers to provide a fix. If the test failures are not resolved within 4 weeks, then the provider will be removed from the list of supported providers.

## Implementing a Provider for Secrets Store CSI Driver

This document highlights the implementation steps for adding a secrets-store-csi-driver provider.

### Implementation details

The driver as of `v0.0.14` adds an option to use gRPC to communicate with the provider. This is an alpha feature and is introduced with a feature flag `--grpc-supported-providers`. The `--grpc-supported-providers` is a `;` delimited list of all providers that support gRPC for communication. This flag will not be necessary after `v0.0.21` since this is the only supported communication mechanism.

> Example usage: `--grpc-supported-providers=provider1;provider2`

To implement a secrets-store-csi-driver provider, you can develop a new provider gRPC server using the stub file available for Go.

- Use the functions and data structures in the stub file: [service.pb.go](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/master/provider/v1alpha1/service.pb.go) to develop the server code
  - The stub file and proto file are shared and hosted in the driver. Vendor-in the stub file and proto file in the provider
  - [fake server example](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/master/provider/fake/fake_server.go)
- Provider runs as a *daemonset* and is deployed on the same host(s) as the secrets-store-csi-driver pods
- Provider Unix Domain Socket volume path. The default volume path for providers is [/etc/kubernetes/secrets-store-csi-providers](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/v0.0.14/deploy/secrets-store-csi-driver.yaml#L88-L89). Add the Unix Domain Socket to the dir in the format `/etc/kubernetes/secrets-store-csi-providers/<provider name>.sock`
- The `<provider name>` in `<provider name>.sock` must match the regular expression `^[a-zA-Z0-9_-]{0,30}$`
- Provider mounts `<kubelet root dir>/pods` (default: [`/var/lib/kubelet/pods`](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/v0.0.14/deploy/secrets-store-csi-driver.yaml#L86-L87)) with [`HostToContainer` mount propagation](https://kubernetes-csi.github.io/docs/deploying.html#driver-volume-mounts) to be able to write the external secrets store content to the volume target path

See [design doc](https://docs.google.com/document/d/10-RHUJGM0oMN88AZNxjOmGz0NsWAvOYrWUEV-FbLWyw/edit?usp=sharing) for more details.

## Features supported by current providers

| Features \ Providers               | Azure | GCP   | AWS   | Vault |
| ---------------------------------- | ----- | ----- | ----- | ----- |
| Sync as Kubernetes secret          | Yes   | Yes   | Yes   | Yes   |
| Rotation                           | Yes   | No    | Yes   | No    |
| Windows                            | Yes   | No    | No    | No    |
| Service account volume projection  | No    | Yes   | Yes   | Yes   |