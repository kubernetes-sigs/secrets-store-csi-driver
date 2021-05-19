It's much easier to debug code with breakpoints. It's also easier to develop new features or make changes to exiting ones with local debug capability. With this in mind, following steps provide a way to setup csi secret store driver for local debugging.

## Prerequisites

For this case we create a kubernetes cluster running locally on our system. Therefore we need the following software:

* [Docker Desktop](https://docs.docker.com/get-docker)
* [kind (Kubernetes in Docker)](https://kind.sigs.k8s.io)
* [Kubectl](https://kubernetes.io/de/docs/tasks/tools/install-kubectl)
* [Visual Studio Code](https://code.visualstudio.com/download)


### Creating a Kubernetes cluster
Replace `hostPath` value in `.local/kind-config.yaml` to match with your local csi driver source code path
``` yaml
# YAML
- hostPath: # /path/to/your/driver/secrets-store-csi-driver/codebase/on/host
```

#### Install nginx-ingress

For port 30123 to work, it is necessary to deploy an nginx controller as an ingress controller:

```sh
kubectl create -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/master/deploy/static/provider/kind/deploy.yaml
```

### Creating a docker image
let's build our docker image from our [Dockerfile](Dockerfile):

```sh
docker build -t debug-driver .
```

After the build is done, load image `debug-driver:latest` on kind cluster:

```sh
kind load docker-image debug-driver:latest
```