#!/bin/bash

azure_client_id=
azure_client_secret=
META_NAME=${META_NAME:-"azure_test"}


jqversion=$(jq --version)
    if [ $? -eq 0 ]; then
        found=$((found + 1))
    else
      apt-get update && apt-get install jq
    fi

##generateSecret
if [ "$azure_client_secret" = "" ]; then
    azure_client_secret=$(openssl rand -base64 24)
    if [ $? -ne 0 ]; then
        echo "Error generating secret"
        exit 1
    fi
fi
echo $azure_client_secret
export AZURE_CLIENT_SECRET=$azure_client_secret


#createApplication
if [ "$azure_client_id" != "" ]; then
    azure_client_id=$(az ad app list --output json | jq -r '.[] | select(.displayName | contains("'$META_NAME'")) .appId')
else
    azure_client_id=$(az ad app create --display-name $META_NAME --identifier-uris http://$META_NAME --homepage http://$META_NAME --password $azure_client_secret --output json | jq -r .appId)
fi

if [ $? -ne 0 ]; then
    echo "Error creating application: $META_NAME @ http://$META_NAME"
    return 1
fi
echo $azure_client_id
export AZURE_CLIENT_ID=$azure_client_id

