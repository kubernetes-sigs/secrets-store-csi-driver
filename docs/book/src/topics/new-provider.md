# Implementing a Provider for Secrets Store CSI Driver

This document highlights the implementation steps for adding a secrets-store-csi-driver provider.

## Implementation details

The driver as of `v0.0.14` adds an option to use gRPC to communicate with the provider. This is an alpha feature and is introduced with a feature flag `--grpc-supported-providers`. The `--grpc-supported-providers` is a `;` delimited list of all providers that support gRPC for communication. The driver will communicate with the provider using gRPC only if the provider name is in the list of supported providers in `--grpc-supported-providers`. 

> Example usage: `--grpc-supported-providers=provider1;provider2`

To implement a secrets-store-csi-driver provider, you can develop a new provider gRPC server using the stub file available for Go.
- Use the functions and data structures in the stub file: [service.pb.go](../provider/v1alpha1/service.pb.go) to develop the server code
  - The stub file and proto file are shared and hosted in the driver. Vendor-in the stub file and proto file in the provider
  - [fake server example](../provider/fake/fake_server.go)
- Provider runs as a daemonset and is deployed on the same host(s) as the secrets-store-csi-driver pods
- Provider Unix Domain Socket volume path. The default volume path for providers is [/etc/kubernetes/secrets-store-csi-driver-providers](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/master/deploy/secrets-store-csi-driver.yaml#L88-L89). Add the Unix Domain Socket to the dir in the format `/etc/kubernetes/secrets-store-csi-driver-providers/<provider name>.sock`
- Provider mounts `<kubelet root dir>/pods` (default: [`/var/lib/kubelet/pods`](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/master/deploy/secrets-store-csi-driver.yaml#L86-L87)) with [`Bidirectional` mount propogation](https://kubernetes-csi.github.io/docs/deploying.html#driver-volume-mounts) to be able to write the external secrets store content to the volume target path

See [design doc](https://docs.google.com/document/d/10-RHUJGM0oMN88AZNxjOmGz0NsWAvOYrWUEV-FbLWyw/edit?usp=sharing) for more details.
