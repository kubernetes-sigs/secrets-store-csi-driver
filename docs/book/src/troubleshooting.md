# Troubleshooting

An overview of a list of components to assist in troubleshooting.

<!-- toc -->

## Logging

To troubleshoot issues with the csi driver, you can look at logs from the `secrets-store` container of the csi driver pod running on the same node as your application pod:

```bash
kubectl get pod -o wide
# find the secrets store csi driver pod running on the same node as your application pod

kubectl logs csi-secrets-store-secrets-store-csi-driver-7x44t secrets-store
```

If the pod fails to start because of the inline volume mount, you can describe the pod to view mount failure errors and events:

```bash
kubectl describe pod <application pod>
```

> It is always a good idea to include relevant logs from csi driver pod when opening a new [issue](https://github.com/kubernetes-sigs/secrets-store-csi-driver/issues).

## pprof

Starting the `secrets-store` container in driver with `--enable-pprof=true` will enable a debug http endpoint at `--pprof-port` (default: 6065). Accessing this will also require `port-forward`:

```bash
kubectl port-forward csi-secrets-store-secrets-store-csi-driver-7x44t secrets-store 6065:6065 &
curl localhost:6065/debug/pprof
```

## Common Errors

### `SecretProviderClass` not found

`kubectl describe pod <application pod>` shows:

```bash
  Warning  FailedMount  3s (x4 over 6s)  kubelet, kind-control-plane  MountVolume.SetUp failed for volume "secrets-store-inline" : rpc error: code = Unknown desc = failed to get secretproviderclass default/azure, error: secretproviderclasses.secrets-store.csi.x-k8s.io "azure" not found
```

The `SecretProviderClass` being referenced in the `volumeMount` needs to exist in the same namespace as the application pod.

### Volume mount fails with `secrets-store.csi.k8s.io` not found in the list of registered CSI drivers

`kubectl describe pod <application pod>` shows:

```bash
  Warning  FailedMount  1s (x4 over 4s)  kubelet, kind-control-plane  MountVolume.SetUp failed for volume "secrets-store-inline" : kubernetes.io/csi: mounter.SetUpAt failed to get CSI client: driver name secrets-store.csi.k8s.io not found in the list of registered CSI drivers
```

Secrets Store CSI Driver is deployed as a *DaemonSet*. The above error indicates the CSI driver pods aren't running on the node.

- If the node is tainted, then add a toleration for the taint in the Secrets Store CSI Driver *DaemonSet*.
- Check to see if there are any node selectors preventing the Secrets Store CSI Driver pods from running on the node.
- Check to see if the `CSIDriver` object has been deployed to the cluster:

  ```bash
  # This is the desired output. If the secrets-store.csi.k8s.io isn't found, then reinstall the driver.
  kubectl get csidriver
  NAME                       ATTACHREQUIRED   PODINFOONMOUNT   MODES       AGE
  secrets-store.csi.k8s.io   false            true             Ephemeral   110m
  ```

### Mount fails with `grpc: received message larger than max`

If the files pulled in by the `SecretProviderClass` are larger than 4MiB you may observe `FailedMount` warnings with a
message that includes `grpc: received message larger than max`. You can configure the driver to accept responses larger
than 4MiB by specifying the `--max-call-recv-msg-size=<size in bytes>` argument to the `secrets-store` container in the
`csi-secrets-store` DaemonSet.

Note that this may also increase memory resource consumption of the `secrets-store` container, so you should also
consider increasing the memory limit as well.

### failed to get CSI client: `driver name secrets-store.csi.k8s.io not found in the list of registered CSI drivers`
### Volume mount fails with `"GRPC error" err="failed to mount objects, error: failed to write file: no such file or directory`
Some Kubernetes distros (such as Rancher and Microk8s) use a custom `kubeletRootDir` path. This may cause errors such as
volume mount failures or failures to register CSI drivers. If the default kubelet directory path of the distro you are
using is not `/var/lib/kubelet`, it can be configured during installation via Helm chart using
`--set linux.kubeletRootDir=<desired/kubelet/dir/path>`. For Rancher the kubelet directory path is
[/opt/rke/var/lib/kubelet](https://github.com/Azure/secrets-store-csi-driver-provider-azure/issues/101#issuecomment-628922356)
and for Microk8s it is
[/var/snap/microk8s/common/var/lib/kubelet](https://github.com/Azure/secrets-store-csi-driver-provider-azure/issues/521#issuecomment-855495247).
