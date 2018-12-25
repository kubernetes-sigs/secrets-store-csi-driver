# Kubernetes-KeyVault-CSI-Driver #

Key Vault CSI driver for Kubernetes - Integrates Key Management Systems with Kubernetes via a CSI driver.  

The Key Vault CSI driver `keyvault.csi.k8s.com` allows Kubernetes to mount multiple secrets, keys, and certs stored in Key Management Systems into their pods as a volume. Once the Volume is attached, the data in it is mounted into the container's file system. 

## Supported Providers
* Azure Key Vault

> 💡 NOTE: To add a new provider, checkout [How to add a new provider](how-to-add-a-new-provider.md).

## Design


## Properties


## How to use ##

### Prerequisites: ### 

💡 Make sure you have a Kubernetes cluster

### Install the KeyVault CSI Driver ###

```bash
kubectl apply -f crd-csi-driver-registry.yaml
kubectl apply -f rbac-csi-driver-registrar.yaml
kubectl apply -f rbac-csi-attacher.yaml
kubectl apply -f csi-keyvault-attacher.yaml
kubectl apply -f keyvault-csi-driver.yaml
```
To validate the installer is running as expected, run the following commands:

```bash
kubectl get po
```

You should see the keyvault CSI driver pods running on each agent node:

```bash
csi-keyvault-2c5ln         2/2     Running   0          4m
csi-keyvault-attacher-0    1/1     Running   0          6m
csi-keyvault-qp9r8         2/2     Running   0          4m
csi-keyvault-zrjt2         2/2     Running   0          4m
```
### Use the KeyVault CSI Driver ###

#### Azure Key Vault Provider ####

The KeyVault CSI driver Azure Key Vault Provider offers two modes for accessing a Key Vault instance: Service Principal and Pod Identity.

##### OPTION 1 - Service Principal #####

Add your service principal credentials as a Kubernetes secrets accessible by the KeyVault CSI driver.

```bash
kubectl create secret generic keyvault-creds --from-literal clientid=<CLIENTID> --from-literal clientsecret=<CLIENTSECRET>
```

Ensure this service principal has all the required permissions to access content in your key vault instance. 
If not, you can run the following using the Azure cli:

```bash
# Assign Reader Role to the service principal for your keyvault
az role assignment create --role Reader --assignee <principalid> --scope /subscriptions/<subscriptionid>/resourcegroups/<resourcegroup>/providers/Microsoft.KeyVault/vaults/<keyvaultname>

az keyvault set-policy -n $KV_NAME --key-permissions get --spn <YOUR SPN CLIENT ID>
az keyvault set-policy -n $KV_NAME --secret-permissions get --spn <YOUR SPN CLIENT ID>
az keyvault set-policy -n $KV_NAME --certificate-permissions get --spn <YOUR SPN CLIENT ID>
```

Fill in the missing pieces in [this](deploy/example/pv-keyvault-csi) deployment to create your own pv, make sure to:

1. reference the service principal kubernetes secret created in the previous step
```yaml
nodePublishSecretRef:
      name: keyvault-creds
```
2. pass in properties for the Key Vault instance to the CSI driver to create a PV and a PVC

|Name|Required|Description|Default Value|
|---|---|---|---|
|usepodidentity|no|specify access mode: service principal or pod identity|"false"|
|keyvaultname|yes|name of KeyVault instance|""|
|keyvaultobjectnames|yes|names of KeyVault objects to access|""|
|keyvaultobjecttypes|yes|types of KeyVault objects: secret, key or cert|""|
|keyvaultobjectversions|no|versions of KeyVault objects, if not provided, will use latest|""|
|resourcegroup|yes|name of resource group containing key vault instance|""|
|subscriptionid|yes|name of subscription containing key vault instance|""|
|tenantid|yes|name of tenant containing key vault instance|""|

keyvaultobjectnames, keyvaultobjecttypes and keyvaultobjectversions are semi-colon (;) separated.

```yaml
csi:
    driver: keyvault.csi.k8s.com
    readOnly: true
    volumeHandle: testfolder
    volumeAttributes:
      usepodidentity: "false"         # [OPTIONAL] if not provided, will default to "false"
      keyvaultname: ""                # the name of the KeyVault
      keyvaultobjectname: ""          # list of KeyVault object names (semi-colon separated)
      keyvaultobjecttype: secret      # list of KeyVault object types: secret, key or cert (semi-colon separated)
      keyvaultobjectversion: ""       # [OPTIONAL] list of KeyVault object versions (semi-colon separated), will get latest if empty
      resourcegroup: ""               # the resource group of the KeyVault
      subscriptionid: ""              # the subscription ID of the KeyVault
      tenantid: ""                    # the tenant ID of the KeyVault
```

Deploy your pv

```bash
kubectl apply -f deploy/example/pv-keyvault-csi.yaml
```

Deploy a static pvc pointing to your pv

```bash
kubectl apply -f deploy/example/pvc-keyvault-csi-static.yaml
```

3. Fill in the missing pieces in [this](deploy/example/nginx-pod-keyvault.yaml) deployment to create your own pod pointing to your PVC, make sure to specify the mount point:
```yaml
volumeMounts:
    - name: keyvault01
      mountPath: "/mnt/keyvault"
```

Example of an nginx pod accessing a secret from a key vault instance:

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: nginx-keyvault
spec:
  containers:
  - image: nginx
    name: nginx-keyvault
    volumeMounts:
    - name: keyvault01
      mountPath: "/mnt/keyvault"
  volumes:
  - name: keyvault01
    persistentVolumeClaim:
      claimName: pvc-keyvault
```

Deploy your app

```bash
kubectl apply -f deploy/example/nginx-pod-keyvault.yaml
```

Validate the pod has access to the secret from key vault:

```bash
kubectl exec -it nginx-flex-kv cat /mnt/keyvault/testsecret
testvalue
```

#### OPTION 2 - Pod identity ####

##### Prerequisites: #####

💡 Make sure you have installed pod identity to your Kubernetes cluster

1. Deploy pod identity components to your cluster
    Follow [these steps](https://github.com/Azure/aad-pod-identity#deploy-the-azure-aad-identity-infra) to install pod identity.

2. Create an Azure User Identity 

    Create an Azure User Identity with the following command. 
    Get `clientId` and `id` from the output. 
    ```
    az identity create -g <resourcegroup> -n <idname>
    ```

3. Assign permissions to new identity
    Ensure your Azure user identity has all the required permissions to read the keyvault instance and to access content within your key vault instance. 
    If not, you can run the following using the Azure cli:

    ```bash
    # Assign Reader Role to new Identity for your keyvault
    az role assignment create --role Reader --assignee <principalid> --scope /subscriptions/<subscriptionid>/resourcegroups/<resourcegroup>/providers/Microsoft.KeyVault/vaults/<keyvaultname>

    # set policy to access keys in your keyvault
    az keyvault set-policy -n $KV_NAME --key-permissions get --spn <YOUR AZURE USER IDENTITY CLIENT ID>
    # set policy to access secrets in your keyvault
    az keyvault set-policy -n $KV_NAME --secret-permissions get --spn <YOUR AZURE USER IDENTITY CLIENT ID>
    # set policy to access certs in your keyvault
    az keyvault set-policy -n $KV_NAME --certificate-permissions get --spn <YOUR AZURE USER IDENTITY CLIENT ID>
    ```

4. Add a new `AzureIdentity` for the new identity to your cluster

    Edit and save this as `aadpodidentity.yaml`

    Set `type: 0` for Managed Service Identity; `type: 1` for Service Principal
    In this case, we are using managed service identity, `type: 0`.
    Create a new name for the AzureIdentity. 
    Set `ResourceID` to `id` of the Azure User Identity created from the previous step.

    ```yaml
    apiVersion: "aadpodidentity.k8s.io/v1"
    kind: AzureIdentity
    metadata:
     name: <any-name>
    spec:
     type: 0
     ResourceID: /subscriptions/<subid>/resourcegroups/<resourcegroup>/providers/Microsoft.ManagedIdentity/userAssignedIdentities/<idname>
     ClientID: <clientid>
    ```

    ```bash
    kubectl create -f aadpodidentity.yaml
    ```

5. Add a new `AzureIdentityBinding` for the new Azure identity to your cluster

    Edit and save this as `aadpodidentitybinding.yaml`
    ```yaml
    apiVersion: "aadpodidentity.k8s.io/v1"
    kind: AzureIdentityBinding
    metadata:
     name: demo1-azure-identity-binding
    spec:
     AzureIdentity: <name_of_AzureIdentity_created_from_previous_step>
     Selector: <label value to match in your app>
    ``` 

    ```
    kubectl create -f aadpodidentitybinding.yaml
    ```
