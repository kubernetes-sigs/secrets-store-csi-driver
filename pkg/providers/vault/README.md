# HashiCorp Vault Provider for Secret Store CSI Driver

HashiCorp [Vault](https://vaultproject.io) provider for Secret Store CSI driver allows you to get secrets stored in
Vault and use the Secret Store CSI driver interface to mount them into Kubernetes pods.

**This is an experimental project. This project isn't production ready.**

## Demo

![Secret Store CSI Driver Vault Provider Demo](./images/secret-store-csi-driver-vault-provider-demo.gif "Secret Store CSI Driver Vault Provider Demo")

## Prerequisites

The guide assumes the following:

* A Kubernetes cluster up and running.
* A Vault cluster up and running. Instructions for spinning up a *development* Vault cluster in Kubernetes can be
found [here](./docs/vault-setup.md).
* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-kubectl) installed.

## Usage

This guide will walk you through the steps to configure and run the Vault provider for Secret Store CSI
driver on Kubernetes.

Make sure you have followed the [prerequisites](#prerequisites) specified above before you continue with this guide.
You should have a development Vault cluster up and running using the [guide](./docs/vault-setup.md) specified above.

### Install the Secrets Store CSI Driver (Kubernetes Version 1.13.x)

**NOTE: If you are using Kubernetes version 1.13.x use the commands below. For Kubernetes version 1.15.x+ use the
guide [here]().**

```bash
kubectl apply -f deploy/crd-csi-driver-registry.yaml
kubectl apply -f deploy/rbac-csi-driver-registrar.yaml
kubectl apply -f deploy/rbac-csi-attacher.yaml
kubectl apply -f deploy/csi-secrets-store-attacher.yaml
kubectl apply -f pkg/providers/vault/examples/secrets-store-csi-driver.yaml
```

To validate the installer is running as expected, run the following commands:

```bash
kubectl get po
```

You should see the Secrets Store CSI driver pods running on each agent node:

```bash
csi-secrets-store-2c5ln         2/2     Running   0          4m
csi-secrets-store-attacher-0    1/1     Running   0          6m
csi-secrets-store-qp9r8         2/2     Running   0          4m
csi-secrets-store-zrjt2         2/2     Running   0          4m
```

### Configure Vault Provider CSI Driver Volume

```bash
vim examples/pv-vault-csi.yaml
```

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-vault
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadOnlyMany
  persistentVolumeReclaimPolicy: Retain
  csi:
    driver: secrets-store.csi.k8s.com
    readOnly: true
    volumeHandle: kv
    volumeAttributes:
      providerName: "vault"
      roleName: "example-role" # Vault role name to perform vault login.
      vaultAddress: "http://10.0.63.109:8200" # Vault API address.
      vaultSkipTLSVerify: "true"
      objects:  |
        array:
          - |
            objectPath: "/path/to/secret"
            objectName: "secret-key-name"
            objectVersion: ""
```

### Create Persistent Volume

```bash
kubectl apply -f examples/pv-vault-csi.yaml
```

### Create Persistent Volume Claim

This `PersistentVolumeClaim` will point to the `PersistentVolume` created
earlier.

```bash
kubectl apply -f examples/pvc-vault-csi-static.yaml
```

### Create an Example Deployment

We will use a NGINX deployment to showcase accessing the secret created by the Secret Store CSI Driver.
The mount point for the secret will be in the [pod deployment specification](./examples/nginx-pod-vault.yaml) file.

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: nginx-vault

.....
    volumeMounts:
    - name: vault01
      mountPath: "/mnt/vault" # Vault mount point.
      readOnly: true
.....

```

Deploy the application

```bash
kubectl apply -f examples/nginx-pod-vault.yaml
``` 

Validate Secret in Pod

```bash
kubectl exec -it nginx-vault cat /mnt/vault/foo
hello
```


### Install the Secrets Store CSI Driver (Kubernetes Version 1.15.x+)

```bash
kubectl apply -f deploy/crd-csi-driver-registry.yaml
kubectl apply -f deploy/rbac-csi-driver-registrar.yaml
kubectl apply -f deploy/rbac-csi-attacher.yaml
kubectl apply -f deploy/csi-secrets-store-attacher.yaml
kubectl apply -f pkg/providers/vault/examples/secrets-store-csi-driver.yaml
```

To validate the installer is running as expected, run the following commands:

```bash
kubectl get po
```

You should see the Secrets Store CSI driver pods running on each agent node:

```bash
csi-secrets-store-2c5ln         2/2     Running   0          4m
csi-secrets-store-attacher-0    1/1     Running   0          6m
csi-secrets-store-qp9r8         2/2     Running   0          4m
csi-secrets-store-zrjt2         2/2     Running   0          4m
```


### Create an Example Deployment

We will use a NGINX deployment to showcase accessing the secret created by the Secret Store CSI Driver.
The mount point and the provider configuration for the secret will be in the [pod deployment specification](./examples/nginx-pod-vault-inline-volume.yaml) file. For this example, we have already configured the Vault provider. 

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: nginx-vault

.....
    volumeMounts:
    - name: secrets-store-inline
      mountPath: "/mnt/secrets-store"
      readOnly: true
  volumes:
    - name: secrets-store-inline
      csi:
        driver: secrets-store.csi.k8s.com
        readOnly: true
        volumeAttributes:
          providerName: "vault"
          roleName: "example-role"
          vaultAddress: "http://10.0.146.70:8200"
          vaultSkipTLSVerify: "true"
          objects:  |
            array:
              - |
                objectPath: "/foo"
                objectName: "bar"
                objectVersion: ""
.....



```

Make sure the `vaultAddress` is pointing to the Kubernetes `vault` service that is created in the prerequisite steps.
You can get the `vault` service address using the following command.

```bash
kubectl get service vault
```

Deploy the application

```bash
kubectl apply -f examples/nginx-pod-vault.yaml
``` 

Validate Secret in Pod

```bash
kubectl exec -it nginx-vault cat /mnt/vault/foo
hello
```
