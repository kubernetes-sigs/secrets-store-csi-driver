#!/bin/bash

# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

keyvault_name=${KEYVAULT_NAME:-csi-secrets-store-e2e}
secret_name=${KEYVAULT_SECRET_NAME:-secret1}
secret_value=${KEYVAULT_SECRET_VALUE:-test}
key_name=${KEYVAULT_KEY_NAME:-key1}
key_version=${KEYVAULT_KEY_VERSION:-"7cc095105411491b84fe1b92ebbcf01a"}
key_value=${KEYVAULT_KEY_VALUE:-"LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUlJQklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUF4K2FadlhJN2FldG5DbzI3akVScgpheklaQ2QxUlBCQVZuQU1XcDhqY05TQk5MOXVuOVJrenJHOFd1SFBXUXNqQTA2RXRIOFNSNWtTNlQvaGQwMFNRCk1aODBMTlNxYkkwTzBMcWMzMHNLUjhTQ0R1cEt5dkpkb01LSVlNWHQzUlk5R2Ywam1ucHNKOE9WbDFvZlRjOTIKd1RINXYyT2I1QjZaMFd3d25MWlNiRkFnSE1uTHJtdEtwZTVNcnRGU21nZS9SL0J5ZXNscGU0M1FubnpndzhRTwpzU3ZMNnhDU21XVW9WQURLL1MxREU0NzZBREM2a2hGTjF5ZHUzbjVBcnREVGI0c0FjUHdTeXB3WGdNM3Y5WHpnClFKSkRGT0JJOXhSTW9UM2FjUWl0Z0c2RGZibUgzOWQ3VU83M0o3dUFQWUpURG1pZGhrK0ZFOG9lbjZWUG9YRy8KNXdJREFRQUIKLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0t"}
resourceGroupName=${RESOUSE_GROUP:-e2etest}
resourceGroupLocation=${LOCATION:-EastUS}
subscription_id=${SUBID:-""}
SPNAME=${SPNAME:-azure_test}


##install jq
jqversion=$(jq --version)
    if [ $? -eq 0 ]; then
        found=$((found + 1))
    else
      apt-get update && apt-get install jq
    fi

#create SP
if [[ -z "$AZURE_CLIENT_ID" ]]; then
    export AZURE_CLIENT_SECRET=$(az ad sp create-for-rbac --skip-assignment --name $SPNAME --output json | jq -r .password)
    export AZURE_CLIENT_ID=$(az ad sp show --id http://$SPNAME --query "appId")
   
    if [ $? -ne 0 ]; then
        echo "Error creating SP: $SPNAME @ http://$SPNAME"
        return 1   
    fi
fi

#If still not found, exit
if [[ -z "${AZURE_CLIENT_ID}" ]] || [[ -z "${AZURE_CLIENT_SECRET}" ]]; then
    echo "Error: Azure service principal is not provided" >&2
    return 1
fi

#Check for existing RG
if [ $(az group exists --name $resourceGroupName) = false ]; then
	az group create --name $resourceGroupName --location $resourceGroupLocation
fi

#Check for KV
kvCheck=$(az keyvault list --query "[?name=='$keyvault_name']")

kevaultExists=($kvCheck.Length -gt 0)

if [ !$kevaultExists ]; then
    #Create Azure KeyVault  
    az keyvault create --name $keyvault_name --resource-group $resourceGroupName --location $resourceGroupLocation
fi

#create keyvault secret
az keyvault secret set --name $secret_name --value $secret_value --vault-name $keyvault_name

#create keyvault key
az keyvault key create --name $key_name --vault-name $keyvault_name

# Assign Reader Role to the service principal for keyvault
az role assignment create --role "Reader" --assignee http://$SPNAME --scope "/subscriptions/$subscription_id/resourcegroups/$resourceGroupName/providers/Microsoft.KeyVault/vaults/$keyvault_name"

#Add policy access 
az keyvault set-policy -n $keyvault_name --key-permissions get --spn http://$SPNAME
az keyvault set-policy -n $keyvault_name --secret-permissions get --spn http://$SPNAME



