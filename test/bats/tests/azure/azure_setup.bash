#!/bin/bash

azure_client_id=${AZURE_CLIENT_ID:-}
azure_client_secret=${AZURE_CLIENT_SECRET:-}
APP_NAME=${APP_NAME:-"azure_test"}

##install jq
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
export AZURE_CLIENT_SECRET=$azure_client_secret


#createApplication
if [ "$azure_client_id" != "" ]; then
    azure_client_id=$(az ad app list --display-name ${ADE_ADAPP_NAME} | jq -r '.[0] | .appId')
else
    azure_client_id=$(az ad app create --display-name $APP_NAME --identifier-uris http://$APP_NAME --homepage http://$APP_NAME --password $azure_client_secret --output json | jq -r .appId)
fi

if [ $? -ne 0 ]; then
    echo "Error creating application: $APP_NAME @ http://$APP_NAME"
    return 1
fi
export AZURE_CLIENT_ID=$azure_client_id
