# Kubernetes-Secrets-Store-CSI-Driver

Secrets Store CSI driver for Kubernetes secrets - Integrates secrets stores with Kubernetes via a CSI volume.  

The Secrets Store CSI driver `secrets-store.csi.k8s.com` allows Kubernetes to mount multiple secrets, keys, and certs stored in enterprise-grade external secrets stores into their pods as a volume. Once the Volume is attached, the data in it is mounted into the container's file system. 

[![CircleCI](https://circleci.com/gh/deislabs/secrets-store-csi-driver/tree/master.svg?style=svg)](https://circleci.com/gh/deislabs/secrets-store-csi-driver/tree/master)

## Features

- Mounts secrets/keys/certs to pod using a CSI volume
- Supports mounting multiple secrets store objects as a single volume
- Supports pod identity to restrict access with specific identities (WIP)
- Supports multiple secrets stores as providers

#### Table of Contents

* [How It Works](#how-it-works)
* [Demo](#demo)
* [Usage](#usage)
* [Providers](#providers)
    + [Azure Key Vault Provider](#azure-key-vault-provider)
    + [Adding a New Provider via the Provider Interface](#adding-a-new-provider-via-the-provider-interface)
* [Testing](#testing)
    + [Unit Tests](#unit-tests)
    + [End-to-end Tests](#end-to-end-tests)
* [Known Issues and Workarounds](#known-issues-and-workarounds)
* [Contributing](#contributing)

## How It Works

The diagram below illustrates how Secrets Store CSI Volume works.

![diagram](img/diagram.png)

## Demo

![Secrets Store CSI Driver Demo](img/demo.gif "Secrets Store CSI Driver Azure Key Vault Provider Demo")

## Usage

Deploy a Kubernetes cluster v1.13.0+ and make sure it's reachable.

### Install the Secrets Store CSI Driver

#### Using Helm Chart

Make sure you already have helm CLI installed.

```bash
$ cd charts/secrets-store-csi-driver
$ helm install . -n csi-secrets-store --namespace dev
```

Expected output:
```console
NAME:   csi-secrets-store
LAST DEPLOYED: Mon Jan  7 18:39:41 2019
NAMESPACE: dev
STATUS: DEPLOYED

RESOURCES:
==> v1/RoleBinding
NAME                   AGE
csi-attacher-role-cfg  1s

==> v1/DaemonSet
csi-secrets-store-secrets-store-csi-driver  1s

==> v1/StatefulSet
csi-secrets-store-attacher  1s

==> v1/Pod(related)

NAME                                    READY  STATUS             RESTARTS  AGE
csi-secrets-store-attacher-0                 0/1    ContainerCreating  0         1s
csi-secrets-store-secrets-store-csi-driver-9crwj  0/2    ContainerCreating  0         1s
csi-secrets-store-secrets-store-csi-driver-pcbtg  0/2    ContainerCreating  0         1s

==> v1beta1/CustomResourceDefinition

NAME                           AGE
csidrivers.csi.storage.k8s.io  1s

==> v1/ClusterRole
driver-registrar-runner   1s
external-attacher-runner  1s

==> v1/ClusterRoleBinding
csi-driver-registrar-role  1s
csi-attacher-role          1s

==> v1/Role
external-attacher-cfg  1s

==> v1/ServiceAccount
csi-driver-registrar  1s
csi-attacher          1s

==> v1/Service
csi-secrets-store-attacher  1s


NOTES:
The Secrets Store CSI Driver is getting deployed to your cluster.

To verify that Secrets Store CSI Driver has started, run:

  kubectl --namespace=dev get pods -l "app=secrets-store-csi-driver"

Now you can follow these steps https://github.com/deislabs/secrets-store-csi-driver#use-the-secrets-store-csi-driver
to create a PersistentVolume, a static PVC, and a deployment using the PVC.

$ kubectl --namespace=dev get pods -l "app=secrets-store-csi-driver"
NAME                                     READY     STATUS    RESTARTS   AGE
csi-secrets-store-attacher-0                  1/1       Running   0          43s
csi-secrets-store-secrets-store-csi-driver-9crwj   2/2       Running   0          43s
csi-secrets-store-secrets-store-csi-driver-pcbtg   2/2       Running   0          43s

```

<details>
<summary><strong>[ALTERNATIVE DEPLOYMENT OPTION] Using Deployment Yamls</strong></summary>

```bash
kubectl apply -f deploy/crd-csi-driver-registry.yaml
kubectl apply -f deploy/rbac-csi-driver-registrar.yaml
kubectl apply -f deploy/rbac-csi-attacher.yaml
kubectl apply -f deploy/csi-secrets-store-attacher.yaml
kubectl apply -f deploy/secrets-store-csi-driver.yaml
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
</details>

### Use the Secrets Store CSI Driver

1. Select a provider from the [list of supported providers](#providers)

2. To create a Secrets Store CSI volume, follow [specific deployment steps](#providers) for the selected provider to update all the required fields in [this deployment yaml](deploy/example/pv-secrets-store-csi.yaml).

```yaml
csi:
  driver: secrets-store.csi.k8s.com
  readOnly: true
  volumeHandle: kv
  volumeAttributes:
    providerName: "azure"
    ...
```
3. Deploy your PersistentVolume (CSI Volume)

```bash
kubectl apply -f deploy/example/pv-secrets-store-csi.yaml
```

4. Deploy a static pvc pointing to your persistentvolume

```bash
kubectl apply -f deploy/example/pvc-secrets-store-csi-static.yaml
```

5. Fill in the missing pieces in [this pod deployment yaml](deploy/example/nginx-pod-secrets-store.yaml) to create your own pod pointing to your PVC. 
Make sure to specify the mount point.

```yaml
volumeMounts:
  - name: secrets-store01
    mountPath: "/mnt/secrets-store"
```

Example of an nginx pod accessing a secret from a PV created by the Secrets Store CSI Driver:

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: nginx-secrets-store
spec:
  containers:
  - image: nginx
    name: nginx-secrets-store
    volumeMounts:
    - name: secrets-store01
      mountPath: "/mnt/secrets-store"
  volumes:
  - name: secrets-store01
    persistentVolumeClaim:
      claimName: pvc-secrets-store
```

Deploy your app

```bash
kubectl apply -f deploy/example/nginx-pod-secrets-store.yaml
```

Validate the pod has access to the secret from your secrets store instance:

```bash
kubectl exec -it nginx-flex-kv cat /mnt/secrets-store/testsecret
testvalue
```

## Providers

This project features a pluggable provider interface developers can implement that defines the actions of the Secrets Store CSI driver.

This enables on-demand retrieval of sensitive objects storied an enterprise-grade external secrets store into Kubernetes while continue to manage these objects outside of Kubernetes.

Each provider may have its own required properties.

Providers must provide the following functionality to be considered a supported integration.
1. Provides the backend plumbing necessary to access objects from the external secrets store.
2. Conforms to the current API provided by the Secrets Store CSI Driver.
3. Does not have access to the Kubernetes APIs and has a well-defined callback mechanism to mount objects to a target path.

### Azure Key Vault Provider

The Secrets Store CSI driver Azure Key Vault Provider offers two modes for accessing a Key Vault instance: Service Principal and Pod Identity.

#### OPTION 1 - Service Principal

Add your service principal credentials as a Kubernetes secrets accessible by the Secrets Store CSI driver.

```bash
kubectl create secret generic secrets-store-creds --from-literal clientid=<CLIENTID> --from-literal clientsecret=<CLIENTSECRET>
```

Ensure this service principal has all the required permissions to access content in your Azure key vault instance. 
If not, you can run the following using the Azure cli:

```bash
# Assign Reader Role to the service principal for your keyvault
az role assignment create --role Reader --assignee <principalid> --scope /subscriptions/<subscriptionid>/resourcegroups/<resourcegroup>/providers/Microsoft.KeyVault/vaults/<keyvaultname>

az keyvault set-policy -n $KV_NAME --key-permissions get --spn <YOUR SPN CLIENT ID>
az keyvault set-policy -n $KV_NAME --secret-permissions get --spn <YOUR SPN CLIENT ID>
az keyvault set-policy -n $KV_NAME --certificate-permissions get --spn <YOUR SPN CLIENT ID>
```

Fill in the missing pieces in [this](deploy/example/pv-secrets-store-csi) deployment to create your own pv, make sure to:

1. reference the service principal kubernetes secret created in the previous step
```yaml
nodePublishSecretRef:
  name: secrets-store-creds
```
2. pass in properties for the Azure Key Vault instance to the Secrets Store CSI driver to create a PV

|Name|Required|Description|Default Value|
|---|---|---|---|
|providerName|yes|specify name of the provider|""|
|usePodIdentity|no|specify access mode: service principal or pod identity|"false"|
|keyvaultName|yes|name of a Key Vault instance|""|
|objects|yes|a string of arrays of strings|""|
|objectName|yes|name of a Key Vault object|""|
|objectType|yes|type of a Key Vault object: secret, key or cert|""|
|objectVersion|no|version of a Key Vault object, if not provided, will use latest|""|
|resourceGroup|yes|name of resource group containing key vault instance|""|
|subscriptionId|yes|subscription ID containing key vault instance|""|
|tenantId|yes|tenant ID containing key vault instance|""|

```yaml
  csi:
    driver: secrets-store.csi.k8s.com
    readOnly: true
    volumeHandle: kv
    volumeAttributes:
      providerName: "azure"
      usePodIdentity: "false"         # [OPTIONAL] default to "false" if empty
      keyvaultName: ""                # name of the KeyVault
      objects:  |
        array:                        # array of objects
          - |
            objectName: secret1
            objectType: secret        # object types: secret, key or cert
            objectVersion: ""         # [OPTIONAL] object versions, default to latest if empty
          - |
            objectName: key1
            objectType: key
            objectVersion: ""
      resourceGroup: ""               # resource group of the KeyVault
      subscriptionId: ""              # subscription ID of the KeyVault
      tenantId: ""                    # tenant ID of the KeyVault
      ...
```

#### OPTION 2 - Pod Identity

_WIP_

### Adding a New Provider via the Provider Interface

Create a new directory for your provider under `providers` and implement the following interface. 
Then add your provider in `providers/register/provider_<provider_name>.go`. Make sure to add a build tag so that
your provider can be excluded from being built. The format for this build tag
should be `no_<provider_name>_provider`. 

```go
// Provider contains the methods required to implement a Secrets Store CSI Driver provider.
type Provider interface {
    // MountSecretsStoreObjectContent mounts content of the secrets store object to target path
    MountSecretsStoreObjectContent(ctx context.Context, attrib map[string]string, secrets map[string]string, targetPath string, permission os.FileMode) error
}
```

## Testing

### Unit Tests

Run unit tests locally with `make test`.

### End-to-end Tests

_WIP_

## Known Issues and Workarounds

_WIP_

## Contributing
