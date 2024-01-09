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

parse_cred() {
    grep -E -o "\b$1[[:blank:]]*=[[:blank:]]*\"[^[:space:]\"]+\"" | cut -d '"' -f 2
}

get_random_region() {
    local REGIONS=("eastus" "eastus2" "southcentralus" "westeurope" "uksouth")
    echo "${REGIONS[${RANDOM} % ${#REGIONS[@]}]}"
}

cleanup() {
    echo "Deleting the AKS cluster ${CLUSTER_NAME}"
    # login again because the bats tests might have logged out after rotating the secret for testing
    az login --service-principal -u "${client_id}" -p "${client_secret}" --tenant "${tenant_id}" > /dev/null
    az account set --subscription "${subscription_id}" > /dev/null
    az group delete --name "${CLUSTER_NAME}" --yes --no-wait || true
}
trap cleanup EXIT

main() {
    # install azure cli
    curl -sL https://aka.ms/InstallAzureCLIDeb | bash > /dev/null

    echo "Logging into Azure"
    az login --service-principal -u "${client_id}" -p "${client_secret}" --tenant "${tenant_id}" > /dev/null
    az account set --subscription "${subscription_id}" > /dev/null

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
        --generate-ssh-keys > /dev/null

    echo "Adding windows nodepool"
    # add windows nodepool
    az aks nodepool add \
        --resource-group "${CLUSTER_NAME}" \
        --cluster-name "${CLUSTER_NAME}" \
        --os-type Windows \
        --name npwin \
        --node-count 1 > /dev/null

    az aks get-credentials --resource-group "${CLUSTER_NAME}" --name "${CLUSTER_NAME}" --overwrite-existing

    if [[ "${REGISTRY}" =~ \.azurecr\.io ]]; then
        az acr login --name "${REGISTRY}"
    fi

    # build the driver image and run e2e tests
    ALL_OS_ARCH=amd64 make e2e-test
}

# for Prow we use the provided AZURE_CREDENTIALS file.
# the file is expected to be in toml format.
if [[ -n "${AZURE_CREDENTIALS:-}" ]]; then
    subscription_id="$(parse_cred SubscriptionID < "${AZURE_CREDENTIALS}")"
    tenant_id="$(parse_cred TenantID < "${AZURE_CREDENTIALS}")"
    client_id="$(parse_cred ClientID < "${AZURE_CREDENTIALS}")"
    client_secret="$(parse_cred ClientSecret < "${AZURE_CREDENTIALS}")"
fi

main
