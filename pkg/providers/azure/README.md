# Azure Key Vault Provider for Secret Store CSI Driver

Azure Key Vault provider for Secret Store CSI driver allows you to get secret contents stored in Azure Key Vault instance and use the Secret Store CSI driver interface to mount them into Kubernetes pods.

## Demo

_WIP_

## Usage

This guide will walk you through the steps to configure and run the Azure Key Vault provider for Secret Store CSI driver on Kubernetes.

## Install the Secrets Store CSI Driver (Kubernetes Version 1.15.x+)
Make sure you have followed the [Installation guide for the Secrets Store CSI Driver](https://github.com/deislabs/secrets-store-csi-driver#usage).


The Azure Key Vault Provider offers two modes for accessing a Key Vault instance: 
1. Service Principal 
1. Pod Identity

## OPTION 1 - Service Principal

1. Add your service principal credentials as a Kubernetes secrets accessible by the Secrets Store CSI driver.

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

1. Update [this sample deployment](examples/v1alpha1_secretproviderclass.yaml) to create a `secretproviderclasses` resource to provide Azure-specific parameters for the Secrets Store CSI driver.

    ```yaml
    apiVersion: secrets-store.csi.k8s.com/v1alpha1
    kind: SecretProviderClass
    metadata:
      name: azure-kvname
    spec:
      provider: azure                   # accepted provider options: azure or vault
      parameters:
        usePodIdentity: "false"         # [OPTIONAL for Azure] if not provided, will default to "false"
        keyvaultName: "kvname"          # the name of the KeyVault
        objects:  |
          array:
            - |
              objectName: secret1
              objectType: secret        # object types: secret, key or cert
              objectVersion: ""         # [OPTIONAL] object versions, default to latest if empty
            - |
              objectName: key1
              objectType: key
              objectVersion: ""
        resourceGroup: "rg1"            # the resource group of the KeyVault
        subscriptionId: "subid"         # the subscription ID of the KeyVault
        tenantId: "tid"                 # the tenant ID of the KeyVault
        cloudName: ""                   # [OPTIONAL] if not provided, will default to AzurePublicCloud

    ```

    | Name           | Required | Description                                                     | Default Value |
    | -------------- | -------- | --------------------------------------------------------------- | ------------- |
    | provider       | yes      | specify name of the provider                                    | ""            |
    | usePodIdentity | no       | specify access mode: service principal or pod identity          | "false"       |
    | keyvaultName   | yes      | name of a Key Vault instance                                    | ""            |
    | objects        | yes      | a string of arrays of strings                                   | ""            |
    | objectName     | yes      | name of a Key Vault object                                      | ""            |
    | objectType     | yes      | type of a Key Vault object: secret, key or cert                 | ""            |
    | objectVersion  | no       | version of a Key Vault object, if not provided, will use latest | ""            |
    | resourceGroup  | yes      | name of resource group containing key vault instance            | ""            |
    | subscriptionId | yes      | subscription ID containing key vault instance                   | ""            |
    | tenantId       | yes      | tenant ID containing key vault instance                         | ""            |
    | cloudName      | no       | azure environment name the subscription belongs to              | ""            |

> Allowed values for cloud name are - `AzurePublicCloud`, `AzureUSGovernmentCloud`, `AzureChinaCloud`, `AzureGermanCloud`

1. Update your [deployment yaml](examples/nginx-pod-secrets-store-inline-volume-secretproviderclass.yaml) to use the Secrets Store CSI driver and reference the `secretProviderClass` resource created in the previous step

    ```yaml
    volumes:
      - name: secrets-store-inline
        csi:
          driver: secrets-store.csi.k8s.com
          readOnly: true
          volumeAttributes:
            secretProviderClass: "azure-kvname"
          nodePublishSecretRef:
            name: secrets-store-creds
    ```

1. Make sure to reference the service principal kubernetes secret created in the previous step

    ```yaml
    nodePublishSecretRef:
      name: secrets-store-creds
    ```

## OPTION 2 - Pod Identity

### Prerequisites:

💡 Make sure you have installed pod identity to your Kubernetes cluster

   __This project makes use of the aad-pod-identity project located  [here](https://github.com/Azure/aad-pod-identity#deploy-the-azure-aad-identity-infra) to handle the identity management of the pods. Reference the aad-pod-identity README if you need further instructions on any of these steps.__

Not all steps need to be followed on the instructions for the aad-pod-identity project as we will also complete some of the steps on our installation here.

1. Install the aad-pod-identity components to your cluster

   - Install the RBAC enabled aad-pod-identiy infrastructure components:
      ```
      kubectl apply -f https://raw.githubusercontent.com/Azure/aad-pod-identity/master/deploy/infra/deployment-rbac.yaml
      ```

   - (Optional) Providing required permissions for MIC

     - If the SPN you are using for the AKS cluster was created separately (before the cluster creation - i.e. not part of the MC_ resource group) you will need to assign it the "Managed Identity Operator" role.
       ```
       az role assignment create --role "Managed Identity Operator" --assignee <sp id> --scope <full id of the managed identity>
       ```

1. Create an Azure User Identity

    Create an Azure User Identity with the following command.
    Get `clientId` and `id` from the output.
    ```
    az identity create -g <resourcegroup> -n <idname>
    ```

1. Assign permissions to new identity
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

1. Add a new `AzureIdentity` for the new identity to your cluster

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

1. Add a new `AzureIdentityBinding` for the new Azure identity to your cluster

    Edit and save this as `aadpodidentitybinding.yaml`
    ```yaml
    apiVersion: "aadpodidentity.k8s.io/v1"
    kind: AzureIdentityBinding
    metadata:
      name: <any-name>
    spec:
      AzureIdentity: <name of AzureIdentity created from previous step>
      Selector: <label value to match in your app>
    ```

    ```
    kubectl create -f aadpodidentitybinding.yaml
    ```

1. Add the following to [this](examples/nginx-pod-secrets-store-inline-volume-secretproviderclass-podid.yaml) deployment yaml:

    a. Include the `aadpodidbinding` label matching the `Selector` value set in the previous step so that this pod will be assigned an identity
    ```yaml
    metadata:
    labels:
      aadpodidbinding: <AzureIdentityBinding Selector created from previous step>
    ```

    b. make sure to update `usepodidentity` to `true`
    ```yaml
    usepodidentity: "true"
    ```
    
1. Update [this sample deployment](examples/v1alpha1_secretproviderclass_podid.yaml) to create a `secretproviderclasses` resource with `usePodIdentity: "true"` to provide Azure-specific parameters for the Secrets Store CSI driver.

1. Deploy your app

    ```bash
    kubectl apply -f examples/nginx-pod-secrets-store-inline-volume-secretproviderclass-podid.yaml
    ```

1. Validate the pod has access to the secret from key vault:

    ```bash
    kubectl exec -it nginx-secrets-store-inline-podid ls /mnt/secrets-store/
    secret1
    ```

**NOTE** When using the `Pod Identity` option mode, there can be some amount of delay in obtaining the objects from keyvault. During the pod creation time, in this particular mode `aad-pod-identity` will need to create the `AzureAssignedIdentity` for the pod based on the `AzureIdentity` and `AzureIdentityBinding`, retrieve token for keyvault. This process can take time to complete and it's possible for the pod volume mount to fail during this time. When the volume mount fails, kubelet will keep retrying until it succeeds. So the volume mount will eventually succeed after the whole process for retrieving the token is complete.
