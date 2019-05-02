# HashiCorp Vault Provider for Secret Store CSI Driver

HashiCorp [Vault](https://vaultproject.io) provider for Secret Store CSI driver allows you to get secrets stored in
Vault and use the Secret Store CSI driver interface to mount them into Kubernetes pods.

**This is an experimental project. This project isn't production ready.**

## Demo

TODO

## Prerequisites

The guide assumes the following:

* A Kubernetes cluster up and running.
* A Vault cluster up and running. Instructions for spinning up a Vault cluster in Kubernetes can be
found [here](#TODO).
* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-kubectl) installed.

## Usage

This guide will walk you through the steps to configure and run the Vault provider for Secret Store CSI
driver on Kubernetes.

### Install the Secrets Store CSI Driver

Follow instructions [here](../../../README.md#Install-the-Secrets-Store-CSI-Driver) to install the Secret
Store CSI driver.

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

### Validate Secret in Pod

```bash
kubectl exec -it nginx-vault cat /mnt/vault/foo
hello
```
