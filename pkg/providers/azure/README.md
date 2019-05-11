# Azure Key Vault Provider for Secret Store CSI Driver
The Azure Key Vault Provider offers two modes for accessing a Key Vault instance: Service Principal and Pod Identity.

## OPTION 1 - Service Principal

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

Fill in the missing pieces in [this](examples/nginx-pod-secrets-store-inline-volume.yaml) deployment to create an inline volume, make sure to:

1. reference the service principal kubernetes secret created in the previous step
```yaml
nodePublishSecretRef:
  name: secrets-store-creds
```
2. pass in properties for the Azure Key Vault instance to the Secrets Store CSI driver to create an inline volume

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

##### Prerequisites: #####

💡 Make sure you have installed pod identity to your Kubernetes cluster

   __This project makes use of the aad-pod-identity project located  [here](https://github.com/Azure/aad-pod-identity#deploy-the-azure-aad-identity-infra) to handle the identity management of the pods. Reference the aad-pod-identity README if you need further instructions on any of these steps.__

Not all steps need to be followed on the instructions for the aad-pod-identity project as we will also complete some of the steps on our installation here.

1. Install the aad-pod-identity components to your cluster
     
   - Install the RBAC enabled aad-pod-identiy infrastructure components:
      ```
      kubectl create -f https://raw.githubusercontent.com/Azure/aad-pod-identity/master/deploy/infra/deployment-rbac.yaml
      ```

   - (Optional) Providing required permissions for MIC

     - If the SPN you are using for the AKS cluster was created separately (before the cluster creation - i.e. not part of the MC_ resource group) you will need to assign it the "Managed Identity Operator" role.
       ```
       az role assignment create --role "Managed Identity Operator" --assignee <sp id> --scope <full id of the managed identity>
       ```

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
     name: <any-name>
    spec:
     AzureIdentity: <name_of_AzureIdentity_created_from_previous_step>
     Selector: <label value to match in your app>
    ``` 

    ```
    kubectl create -f aadpodidentitybinding.yaml
    ```

6. Add the following to [this](examples/nginx-pod-secrets-store-inline-volume.yaml) deployment yaml:

    a. Include the `aadpodidbinding` label matching the `Selector` value set in the previous step so that this pod will be assigned an identity
    ```yaml
    metadata:
    labels:
        aadpodidbinding: "NAME OF the AzureIdentityBinding SELECTOR"
    ```

    b. make sure to update `usepodidentity` to `true`
    ```yaml
    usepodidentity: "true"
    ```

7. Deploy your app

    ```bash
    kubectl create -f examples/nginx-pod-secrets-store-inline-volume.yaml
    ```

8. Validate the pod has access to the secret from key vault:

    ```bash
    kubectl exec -it nginx-secrets-store-inline-pod-identity cat /kvmnt/testsecret
    testvalue
    ```

**NOTE** When using the `Pod Identity` option mode, there can be some amount of delay in obtaining the objects from keyvault. During the pod creation time, in this particular mode `aad-pod-identity` will need to create the `AzureAssignedIdentity` for the pod based on the `AzureIdentity` and `AzureIdentityBinding`, retrieve token for keyvault. This process can take time to complete and it's possible for the pod volume mount to fail during this time. When the volume mount fails, kubelet will keep retrying until it succeeds. So the volume mount will eventually succeed after the whole process for retrieving the token is complete.