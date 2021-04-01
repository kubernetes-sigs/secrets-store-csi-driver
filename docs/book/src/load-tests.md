# Load tests

This document highlights the results from load tests using secrets-store-csi-driver `v0.0.21`.

> Note: Refer to [doc](https://docs.google.com/document/d/1ba8gTC-i33Df6uiOB8rW8jBX2B0lK8LszUjYlPzNwlQ/edit?usp=sharing) for more details on the optimization done as part of `v0.0.21` release.

The results posted here can be used as a guide for configuring resource and memory limits for the CSI driver **daemonset** pods.

## Testing Environment

1. 250 nodes [Azure Kubernetes Service](https://azure.microsoft.com/en-us/services/kubernetes-service/) cluster
    - VM Size: [Standard_DS2_v2](https://docs.microsoft.com/en-us/azure/virtual-machines/dv2-dsv2-series#dsv2-series) (2 vCPU, 7GiB)
2. 3500 Kubernetes secrets in the cluster
    - These secrets were pre-configured to ensure existing Kubernetes secrets doesn't impact the memory for the CSI driver.
3. 7250 pods running in the cluster
    - These pods were pre-configured to ensure existing Kubernetes pods doesn't impact the memory for the CSI driver.

The Secrets Store CSI Driver and [Azure Keyvault Provider](https://azure.github.io/secrets-store-csi-driver-provider-azure/) were deployed to the cluster.

Secrets Store CSI Driver features enabled:

1. [Sync as Kubernetes secret](./topics/sync-as-kubernetes-secret.md)
2. [Secret Auto rotation](./topics/secret-auto-rotation.md)
    - Rotation Poll Interval: 2m

## Testing scenarios

### 10000 pods with CSI volume

1. 10000 pods with CSI volume.
    - Total number of pods in the cluster = 7250 + 10000 = 17250 pods.
2. `SecretProviderClass` with syncing 2 Kubernetes secrets.

```bash
➜ kubectl top pods -l app=csi-secrets-store -n kube-system --sort-by=memory
NAME                      CPU(cores)   MEMORY(bytes)
csi-secrets-store-kd2bc   3m           54Mi
csi-secrets-store-wx6z9   3m           52Mi
csi-secrets-store-6gjqq   3m           52Mi
csi-secrets-store-knl5g   4m           52Mi
csi-secrets-store-9lzzn   4m           51Mi
```

The current default memory and resource limits have been configured based on the above tests.

## Understanding Secrets Store CSI Driver memory consumption

As of Secrets Store CSI Driver `v0.0.21`, the memory consumption for the driver is dependent on:

1. Number of pods on the same node as the driver pod.
2. Number of secrets with
    a. `secrets-store.csi.k8s.io/managed=true` label. This label is set for all the secrets created by the Secrets Store CSI Driver.
    b. `secrets-store.csi.k8s.io/used=true` label. This label needs to be set for all `nodePublishSecretRef`.
3. Number of `SecretProviderClass` across all namespaces.
4. Number of `SecretProviderClassPodStatus` created by Secrets Store CSI Driver for the pod on the same node as the application pod.
   a. Secrets Store CSI Driver creates a `SecretProviderClassPodStatus` to map pod to `SecretProviderClass`. See [doc](./concepts.md#secretproviderclasspodstatus) for more details.

<aside class="note warning">
<h1>Warning</h1>

If the secret rotation feature is enabled and filtered secret watch is not enabled, it'll cache Kubernetes secrets across all namespaces. To only cache the secrets with the above 2 labels:

1. Label all existing `nodePublishSecretRef` secrets with `secrets-store.csi.k8s.io/used=true` by running `kubectl label secret <node publish secret ref name> secrets-store.csi.k8s.io/used=true`.
2. Enable filtered secret watch by setting `--filtered-watch-secret=true` in `secrets-store` container or via helm using `--set filteredWatchSecret=true`.

**NOTE:** `--filtered-watch-secret=true` will be enabled by default in n+3 releases (`v0.0.25`). Please take the necessary action to label the `nodePublishSecretRef` secrets with the `secrets-store.csi.k8s.io/used=true` label.
</aside>
