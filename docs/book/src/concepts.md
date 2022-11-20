# Concepts

<!-- toc -->

## How it works

The diagram below illustrates how Secrets Store CSI volume works:

![diagram](./images/diagram.png)

Similar to Kubernetes secrets, on pod start and restart, the Secrets Store CSI driver communicates with the provider using gRPC to retrieve the secret content from the external Secrets Store specified in the `SecretProviderClass` custom resource. Then the volume is mounted in the pod as `tmpfs` and the secret contents are written to the volume.

On pod delete, the corresponding volume is cleaned up and deleted.

## Secrets Store CSI Driver

The Secrets Store CSI Driver is a **daemonset** that facilitates communication with every instance of Kubelet. Each driver pod has the following containers:

- `node-driver-registrar`: Responsible for registering the CSI driver with Kubelet so that it knows which unix domain socket to issue the CSI calls on. This sidecar container is provider by the Kubernetes CSI team. See [doc](https://kubernetes-csi.github.io/docs/node-driver-registrar.html) for more details.
- `secrets-store`: Implements the CSI `Node` service gRPC services described in the CSI specification. It's responsible for mount/unmount the volumes during pod creation/deletion. This component is developed and maintained in this [repo](https://github.com/kubernetes-sigs/secrets-store-csi-driver).
- `liveness-probe`: Responsible for monitoring the health of the CSI driver and reports to Kubernetes. This enables Kubernetes to automatically detect issues with the driver and restart the pod to try and fix the issue. This sidecar container is provider by the Kubernetes CSI team. See [doc](https://kubernetes-csi.github.io/docs/livenessprobe.html) for more details.

## Provider for the Secrets Store CSI Driver

The CSI driver communicates with the provider using gRPC to fetch the mount contents from external Secrets Store. Refer to [doc](./providers.md) for more details on the how to implement a provider for the driver and criteria for supported providers.

Currently supported providers:

- [AWS Provider](https://github.com/aws/secrets-store-csi-driver-provider-aws)
- [Azure Provider](https://azure.github.io/secrets-store-csi-driver-provider-azure/)
- [GCP Provider](https://github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp)
- [Vault Provider](https://github.com/hashicorp/secrets-store-csi-driver-provider-vault)

## Security

The Secrets Store CSI Driver **daemonset** runs as `root` in a `privileged` pod. This is because the **daemonset** is
responsible for creating new `tmpfs` filesystems and `mount`ing them into existing pod filesystems within the node's
`hostPath`. `root` is necessary for the `mount` syscall and other filesystem operations and `privileged` is required for
to use `mountPropagation: Bidirectional` to modify other running pod's filesystems.

The provider plugins are also required to run as `root` (though `privileged` should not be necessary). This is because
the provider plugin must create a unix domain socket in a `hostPath` for the driver to connect to.

Further, service account tokens for pods that require secrets may be forwarded from the kubelet process to the driver
and then to provider plugins. This allows the provider to impersonate the pod when contacting the external secret API.

**Note:** On Windows hosts secrets will be written to the the node's filesystem which may be persistent storage. This
contrasts with Linux where a `tmpfs` is used to try to ensure that secret material is never persisted.

**Note:** Kubernetes 1.22 introduced a way to configure nodes to
[use swap memory](https://kubernetes.io/blog/2021/08/09/run-nodes-with-swap-alpha/), however if this is used then secret
material may be persisted to the node's disk. To ensure that secrets are not written to persistent disk ensure
`failSwapOn` is set to `true` (which is the default).

## Custom Resource Definitions (CRDs)

### SecretProviderClass

The `SecretProviderClass` is a namespaced resource in Secrets Store CSI Driver that is used to provide driver configurations and provider-specific parameters to the CSI driver.

`SecretProviderClass` custom resource should have the following components:

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: my-provider
spec:
  provider: vault                             # accepted provider options: azure or vault or gcp
  parameters:                                 # provider-specific parameters
```

> Refer to the provider docs for required provider specific parameters.

Here is an example of a `SecretProviderClass` resource:

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: my-provider
  namespace: default
spec:
  provider: azure
  parameters:
    usePodIdentity: "false"
    useManagedIdentity: "false"
    keyvaultName: "$KEYVAULT_NAME"
    objects: |
      array:
        - |
          objectName: $SECRET_NAME
          objectType: secret
          objectVersion: $SECRET_VERSION
        - |
          objectName: $KEY_NAME
          objectType: key
          objectVersion: $KEY_VERSION
    tenantId: "$TENANT_ID"
```

Reference the `SecretProviderClass` in the pod volumes when using the CSI driver:

```yaml
volumes:
  - name: secrets-store-inline
    csi:
      driver: secrets-store.csi.k8s.io
      readOnly: true
      volumeAttributes:
        secretProviderClass: "my-provider"
```

> NOTE: The `SecretProviderClass` needs to be created in the same namespace as the pod.

### SecretProviderClassPodStatus

The `SecretProviderClassPodStatus` is a namespaced resource in Secrets Store CSI Driver that is created by the CSI driver to track the binding between a pod and `SecretProviderClass`. The `SecretProviderClassPodStatus` contains details about the current object versions that have been loaded in the pod mount.

The `SecretProviderClassPodStatus` is created by the CSI driver in the same namespace as the pod and `SecretProviderClass` with the name `<pod name>-<namespace>-<secretproviderclass name>`.

Here is an example of a `SecretProviderClassPodStatus` resource:

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClassPodStatus
metadata:
  creationTimestamp: "2021-01-21T19:20:11Z"
  generation: 1
  labels:
    internal.secrets-store.csi.k8s.io/node-name: kind-control-plane
    manager: secrets-store-csi
    operation: Update
    time: "2021-01-21T19:20:11Z"
  name: nginx-secrets-store-inline-crd-dev-azure-spc
  namespace: dev
  ownerReferences:
  - apiVersion: v1
    kind: Pod
    name: nginx-secrets-store-inline-crd
    uid: 10f3e31c-d20b-4e46-921a-39e4cace6db2
  resourceVersion: "1638459"
  selfLink: /apis/secrets-store.csi.x-k8s.io/v1/namespaces/dev/secretproviderclasspodstatuses/nginx-secrets-store-inline-crd
  uid: 1d078ad7-c363-4147-a7e1-234d4b9e0d53
status:
  mounted: true
  objects:
  - id: secret/secret1
    version: c55925c29c6743dcb9bb4bf091be03b0
  - id: secret/secret2
    version: 7521273d0e6e427dbda34e033558027a
  podName: nginx-secrets-store-inline-crd
  secretProviderClassName: azure-spc
  targetPath: /var/lib/kubelet/pods/10f3e31c-d20b-4e46-921a-39e4cace6db2/volumes/kubernetes.io~csi/secrets-store-inline/mount
```

The pod for which the `SecretProviderClassPodStatus` was created is set as owner. When the pod is deleted, the `SecretProviderClassPodStatus` resources associated with the pod get automatically deleted.
