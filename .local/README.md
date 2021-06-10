# Overview
It's much easier to debug code with breakpoints while developing new features or making changes to existing codebase. With this in mind, following steps provides a way to setup csi secret store driver for local debugging.

> NOTE: Steps in this guide are not tested by CI/CD. This is just one of the way to locally debug the code and a good starting point.

## Prerequisites

* [Docker Desktop](https://docs.docker.com/get-docker)
* [kind (Kubernetes in Docker)](https://kind.sigs.k8s.io)
* [Kubectl](https://kubernetes.io/de/docs/tasks/tools/install-kubectl)
* [Visual Studio Code](https://code.visualstudio.com/download)
* [GOLang](https://golang.org/doc/install)
* [VSCode GO extension](https://marketplace.visualstudio.com/items?itemName=golang.Go)


### Creating local Kubernetes cluster
- Replace `hostPath` value in [kind-config.yaml](kind-config.yaml) to match with your local csi driver source code path
``` yaml
# YAML
- hostPath: # /path/to/your/driver/secrets-store-csi-driver/codebase/on/host
```
- Create Kind cluster:
```sh
kind create cluster --config .local/kind-config.yaml
```


### Creating a docker image
- Build docker image from [Dockerfile](Dockerfile):

```sh
docker build -t debug-driver -f .local/Dockerfile .
```

- Load image `debug-driver:latest` on kind cluster:

```sh
kind load docker-image debug-driver:latest
```

### Deploy resources for debugging
- Deploy following Driver resources:
```sh
kubectl apply -f deploy/rbac-secretproviderclass.yaml
kubectl apply -f deploy/csidriver.yaml
kubectl apply -f deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml
kubectl apply -f deploy/secrets-store.csi.x-k8s.io_secretproviderclasspodstatuses.yaml
kubectl apply -f deploy/rbac-secretprovidersyncing.yaml

# deploy your specific provider
```

- Deploy pv and pvc to mount codebase into the cluster:
```sh
kubectl apply -f .local/persistent-volume.yaml
```

- Deploy driver:
```sh
kubectl apply -f .local/debug-driver.yaml
```
- Check the logs of debug-driver pod to make sure `dlv` API server is listening:
```
API server listening at: [::]:30123
```

### launch.json configuration
Use following `launch.json` configuration to attach debugger.
```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Driver debug",
            "type": "go",
            "request": "attach",
            "mode":"remote",
            "remotePath": "/secrets-store-csi-driver-codebase",
            "port": 30123,
            "host": "127.0.0.1",
            "showLog": true
        }
    ]
}
```
Happy Debugging..

## Cleanup
```sh
kind delete cluster
```
