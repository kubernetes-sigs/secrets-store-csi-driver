# secrets-store-csi-driver

## Installation

Quick start instructions for the setup and configuration of secrets-store-csi-driver using Helm.

### Prerequisites

- [Helm v3.0+](https://helm.sh/docs/intro/quickstart/#install-helm)

### Installing the chart

```bash
$ helm repo add secrets-store-csi-driver https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/master/charts
$ helm install csi-secrets-store secrets-store-csi-driver/secrets-store-csi-driver
```

### Configuration

The following table lists the configurable parameters of the csi-secrets-store-provider-azure chart and their default values.

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `nameOverride` | String to partially override secrets-store-csi-driver.fullname template with a string (will prepend the release name) | `""` |
| `fullnameOverride` | String to fully override secrets-store-csi-driver.fullname template with a string | `""` |
| `linux.image.repository` | Linux image repository | `us.gcr.io/k8s-artifacts-prod/csi-secrets-store/driver` |
| `linux.image.pullPolicy` | Linux image pull policy | `Always` |
| `linux.image.tag` | Linux image tag | `v0.0.12` |
| `linux.enabled` | Install secrets store csi driver on linux nodes | true |
| `linux.kubeletRootDir` | Configure the kubelet root dir | `/var/lib/kubelet` |
| `linux.nodeSelector` | Node Selector for the daemonset on linux nodes | `{}` |
| `linux.metricsAddr` | The address the metric endpoint binds to | `:8080` |
| `windows.image.repository` | Windows image repository | `us.gcr.io/k8s-artifacts-prod/csi-secrets-store/driver` |
| `windows.image.pullPolicy` | Windows image pull policy | `IfNotPresent` |
| `windows.image.tag` | Windows image tag | `v0.0.12` |
| `windows.enabled` | Install secrets store csi driver on windows nodes | false |
| `windows.kubeletRootDir` | Configure the kubelet root dir | `C:\var\lib\kubelet` |
| `windows.nodeSelector` | Node Selector for the daemonset on windows nodes | `{}` |
| `windows.metricsAddr` | The address the metric endpoint binds to | `:8080` |
| `logLevel.debug` | Enable debug logging | true |
| `livenessProbe.port` | Liveness probe port | `9808` |
| `livenessProbe.logLevel` | Liveness probe container logging verbosity level | `2` |
| `rbac.install` | Install default rbac roles and bindings | true |
| `minimumProviderVersions` | A comma delimited list of key-value pairs of minimum provider versions with driver | `""` |
