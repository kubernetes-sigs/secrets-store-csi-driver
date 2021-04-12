# Upgrades

This page includes instructions for upgrading the driver to the latest version.

```bash
helm upgrade csi-secrets-store secrets-store-csi-driver/secrets-store-csi-driver --namespace=NAMESPACE
```

Set `NAMESPACE` to the same namespace where the driver was originally installed,
(i.e. `kube-system`)

If you are upgrading from one of the following versions there may be additional
steps that you should take.

## pre `v0.0.20`

`v0.0.20` removed support for non-gRPC based providers. Follow your provider
documentation to upgrade providers to use gRPC before upgrading the driver to
`v0.0.20` or greater.

## pre `v0.0.18`

`v0.0.17` and earlier installed the driver to the `default` namespace when using
the YAML based install. Newer versions of the driver YAML files install the
driver to the `kube-system` namespace. After applying the new YAML files to your
cluster run the following to clean up old resources:

```bash
kubectl delete daemonset csi-secrets-store --namespace=default
kubectl delete daemonset csi-secrets-store-windows --namespace=default
kubectl delete serviceaccount secrets-store-csi-driver --namespace=default
```

## pre `v0.0.12`

The `SecretProviderClass` needs to be in the same namespace as the pod
referencing it as of `v0.0.12`.

Defining driver configuration and provider-specific parameters to the CSI driver
in `pod.Spec[].Volumes` has been deprecated in `v0.0.12`. It is now mandatory to
use `SecretProviderClass` custom resource.
