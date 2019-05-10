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

_WIP_