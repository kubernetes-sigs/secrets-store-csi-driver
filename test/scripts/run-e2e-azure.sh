#!/usr/bin/env bash

# Copyright 2024 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

: "${REGISTRY:?Environment variable empty or not defined.}"

readonly CLUSTER_NAME="${CLUSTER_NAME:-sscsi-e2e-$(openssl rand -hex 2)}"
readonly KEYVAULT_NAME="secrets-store-csi-e2e"

IMAGE_VERSION=e2e-$(git rev-parse --short HEAD)
IMAGE_TAG=${REGISTRY}/driver:${IMAGE_VERSION}

get_random_region() {
    local REGIONS=("eastus" "eastus2" "uksouth")
    echo "${REGIONS[${RANDOM} % ${#REGIONS[@]}]}"
}

cleanup() {
    echo "Cleaning up resources"
    az login --service-principal -u "${AZURE_CLIENT_ID}" --t "${AZURE_TENANT_ID}" --federated-token "$(cat "${AZURE_FEDERATED_TOKEN_FILE}")" > /dev/null
    az account set --subscription "${AZURE_SUBSCRIPTION_ID}" > /dev/null

    # Clean up role assignment if IDENTITY_OBJECT_ID is set
    if [[ -n "${IDENTITY_OBJECT_ID:-}" ]]; then
        echo "Removing role assignment from Key Vault"
        KEYVAULT_RESOURCE_ID=$(az keyvault show --name "${KEYVAULT_NAME}" --query "id" -otsv 2>/dev/null || true)
        if [[ -n "${KEYVAULT_RESOURCE_ID}" ]]; then
            az role assignment delete \
                --role "Key Vault Secrets User" \
                --assignee "${IDENTITY_OBJECT_ID}" \
                --scope "${KEYVAULT_RESOURCE_ID}" > /dev/null 2>&1 || true
        fi
    fi

    echo "Deleting the AKS cluster ${CLUSTER_NAME}"
    az group delete --name "${CLUSTER_NAME}" --yes --no-wait || true
}
trap cleanup EXIT

main() {
    # install azure cli
    curl -sL https://aka.ms/InstallAzureCLIDeb | bash > /dev/null

    echo "Logging into Azure"
    az login --service-principal -u "${AZURE_CLIENT_ID}" --t "${AZURE_TENANT_ID}" --federated-token "$(cat "${AZURE_FEDERATED_TOKEN_FILE}")" > /dev/null
    az account set --subscription "${AZURE_SUBSCRIPTION_ID}" > /dev/null

    LOCATION=$(get_random_region)
    echo "Creating AKS cluster ${CLUSTER_NAME} in ${LOCATION}"
    az group create --name "${CLUSTER_NAME}" --location "${LOCATION}" > /dev/null
    az aks create \
        --resource-group "${CLUSTER_NAME}" \
        --name "${CLUSTER_NAME}" \
        --node-count 1 \
        --node-vm-size Standard_DS3_v2 \
        --enable-managed-identity \
        --network-plugin azure \
        --enable-oidc-issuer \
        --generate-ssh-keys > /dev/null

    # only add windows pool if TEST_WINDOWS is set and equal to true
    if [[ "${TEST_WINDOWS:-}" == "true" ]]; then
        echo "Adding windows nodepool"
        # add windows nodepool with explicit x64 VM size (Windows doesn't support ARM64)
        az aks nodepool add \
            --resource-group "${CLUSTER_NAME}" \
            --cluster-name "${CLUSTER_NAME}" \
            --os-type Windows \
            --name npwin \
            --node-count 1 \
            --node-vm-size Standard_D2s_v3 > /dev/null
    fi

    az aks get-credentials --resource-group "${CLUSTER_NAME}" --name "${CLUSTER_NAME}" --overwrite-existing

    # confirm the cluster is up and running
    kubectl get nodes -o wide
    kubectl get pods -A

    if [[ "${REGISTRY}" =~ \.azurecr\.io ]]; then
        az acr login --name "${REGISTRY}"
    fi

    AKS_CLUSTER_OIDC_ISSUER_URL=$(az aks show -g "${CLUSTER_NAME}" -n "${CLUSTER_NAME}" --query "oidcIssuerProfile.issuerUrl" -otsv)
    # Create managed identity that'll be used by the provider to access keyvault
    echo "Creating managed identity"
    user_assigned_identity_name="sscsi-e2e-$(openssl rand -hex 2)"
    az identity create --resource-group "${CLUSTER_NAME}" --name "${user_assigned_identity_name}" > /dev/null
    IDENTITY_CLIENT_ID=$(az identity show --resource-group "${CLUSTER_NAME}" --name "${user_assigned_identity_name}" --query 'clientId' -otsv)
    export IDENTITY_CLIENT_ID
    IDENTITY_OBJECT_ID=$(az identity show --resource-group "${CLUSTER_NAME}" --name "${user_assigned_identity_name}" --query 'principalId' -otsv)

    # Create the federated identity credential (FIC) for the managed identity
    echo "Creating federated identity credential for default:default"
    az identity federated-credential create --name "kubernetes-federated-credential-default" \
        --identity-name "${user_assigned_identity_name}" \
        --resource-group "${CLUSTER_NAME}" \
        --issuer "${AKS_CLUSTER_OIDC_ISSUER_URL}" \
        --subject "system:serviceaccount:default:default" \
        --audiences api://AzureADTokenExchange > /dev/null

    echo "Creating federated identity credential for test-ns:default"
    az identity federated-credential create --name "kubernetes-federated-credential-test-ns" \
        --identity-name "${user_assigned_identity_name}" \
        --resource-group "${CLUSTER_NAME}" \
        --issuer "${AKS_CLUSTER_OIDC_ISSUER_URL}" \
        --subject "system:serviceaccount:test-ns:default" \
        --audiences api://AzureADTokenExchange > /dev/null

    echo "Creating federated identity credential for negative-test-ns:default"
    az identity federated-credential create --name "kubernetes-federated-credential-negative-test-ns" \
        --identity-name "${user_assigned_identity_name}" \
        --resource-group "${CLUSTER_NAME}" \
        --issuer "${AKS_CLUSTER_OIDC_ISSUER_URL}" \
        --subject "system:serviceaccount:negative-test-ns:default" \
        --audiences api://AzureADTokenExchange > /dev/null

    # Assigning the managed identity the necessary permissions to access the keyvault using RBAC
    echo "Assigning managed identity Key Vault Secrets User role on keyvault"
    KEYVAULT_RESOURCE_ID=$(az keyvault show --name "${KEYVAULT_NAME}" --query "id" -otsv)
    # Use --assignee-principal-type to avoid replication delay issues
    # See: https://learn.microsoft.com/en-us/azure/role-based-access-control/troubleshooting#symptom---assigning-a-role-to-a-new-principal-sometimes-fails
    az role assignment create \
        --role "Key Vault Secrets User" \
        --assignee-object-id "${IDENTITY_OBJECT_ID}" \
        --assignee-principal-type ServicePrincipal \
        --scope "${KEYVAULT_RESOURCE_ID}" > /dev/null

    docker pull "${IMAGE_TAG}" || ALL_ARCH_linux=amd64 make container-all push-manifest
    make e2e-install-prerequisites
    
    if [[ ${RELEASE:-} == "true" ]]; then
        make e2e-helm-deploy-release
    else
        make e2e-helm-deploy
    fi

    # Run the e2e tests
    make e2e-azure
}

main
