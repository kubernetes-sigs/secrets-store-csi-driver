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

: "${GOOGLE_APPLICATION_CREDENTIALS:?Environment variable empty or not defined.}"

readonly CLUSTER_NAME="secret-provider-cluster-gcp-$(openssl rand -hex 4)"

function boskosctlwrapper() {
  boskosctl --server-url http://"${BOSKOS_HOST}" --owner-name "cluster-api-provider-gcp" "${@}"
}

cleanup() {

    gcloud container clusters delete --location us-central1-c ${CLUSTER_NAME}
    # stop boskos heartbeat
    if [ -n "${BOSKOS_HOST:-}" ]; then
        boskosctlwrapper release --name "${ }" --target-state dirty
    fi

}
trap cleanup EXIT

main() {
    echo "starting the script"

    if [[ -z "$(command -v boskosctl)" ]]; then
        echo "installing boskosctl"
        GO111MODULE=on go install sigs.k8s.io/boskos/cmd/boskosctl@master
        echo "'boskosctl' has been installed to $GOPATH/bin, make sure this directory is in your \$PATH"
    fi

    echo "testing boskosctl"
    boskosctl --help

    if [ -n "${BOSKOS_HOST:-}" ]; then
        echo "Boskos acquire - ${BOSKOS_HOST}"
        export BOSKOS_RESOURCE="$( boskosctlwrapper acquire --type gce-project --state free --target-state busy --timeout 1h )"
        export RESOURCE_NAME=$(echo $BOSKOS_RESOURCE | jq  -r ".name")
        export GCP_PROJECT=$(echo $BOSKOS_RESOURCE | jq  -r ".name")

        # send a heartbeat in the background to keep the lease while using the resource
        echo "Starting Boskos HeartBeat"
        boskosctlwrapper heartbeat --resource "${BOSKOS_RESOURCE}" &
    fi

    if [[ -z "$GOOGLE_APPLICATION_CREDENTIALS" ]]; then
        echo "GOOGLE_APPLICATION_CREDENTIALS is not set. Please set this to the path of the service account used to run this script."
    else
        gcloud auth activate-service-account --key-file="${GOOGLE_APPLICATION_CREDENTIALS}"
    fi
    # GCP_PROJECT=$(jq -r .project_id "${GOOGLE_APPLICATION_CREDENTIALS}")
    echo "Using project ${GCP_PROJECT}"

    gcloud projects describe ${GCP_PROJECT}

    gcloud config set project ${GCP_PROJECT}

    echo "creating cluster..."
    gcloud container clusters create ${CLUSTER_NAME} --location=us-central1-c --workload-pool=${GCP_PROJECT}.svc.id.goog

    make e2e-helm-deploy e2e-gcp


}

main