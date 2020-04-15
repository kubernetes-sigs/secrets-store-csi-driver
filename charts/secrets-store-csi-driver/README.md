# secrets-store-csi-driver

## Installation with Helm 3

Quick start instructions for the setup and configuration for secrets-store-csi-driver using Helm.

### Prerequisites

- [Helm v3.0+](https://helm.sh/docs/intro/quickstart/#install-helm)

### Install charts

**Get the source**
```bash
$ go get -d sigs.k8s.io/secrets-store-csi-driver
$ cd "$(go env GOPATH)/src/sigs.k8s.io/secrets-store-csi-driver"
```

**Create the desired namespace if not exists**
```bash
NAMESPACE=csi-secrets-store
kubectl create ns $NAMESPACE
```

**Install on linux only cluster**
```bash
$ helm install csi-secrets-store charts/secrets-store-csi-driver -n $NAMESPACE
```

**Install on windows only cluster**
```bash
$ helm install csi-secrets-store charts/secrets-store-csi-driver --set linux.enabled=false --set windows.enabled=true -n $NAMESPACE
```

**Install on linux and windows hybrid cluster**
```bash
$ helm install csi-secrets-store charts/secrets-store-csi-driver --set windows.enabled=true -n $NAMESPACE
```

### Uninstall

To uninstall/delete the last deployment:

```bash
$ helm ls
$ helm delete csi-secrets-store
``` 