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

function boskosctlwrapper() {
 boskosctl --server-url http://"${BOSKOS_HOST}" --owner-name "secret-store-provider-gcp" "${@}"
}

cleanup() {
    gcloud beta secrets delete ${SECRET_ID} --quiet
    # stop boskos heartbeat
    if [ -n "${BOSKOS_HOST:-}" ]; then
        boskosctlwrapper release --name "${ }" --target-state dirty
    fi
}
trap cleanup EXIT



main() {
    echo "starting the secret store csi driver test for gcp provider"
    # TODOs
    # 1. Create a temporary secret in boskos pool once https://github.com/kubernetes/k8s.io/pull/7416 is submitted.
    # 2. Rotate secrets created in above step
    # 3. Clean up the secret.

    #install boskosctl
    if [[ -z "$(command -v boskosctl)" ]]; then
        echo "installing boskosctl"
        GO111MODULE=on go install sigs.k8s.io/boskos/cmd/boskosctl@master
        echo "'boskosctl' has been installed to $GOPATH/bin, make sure this directory is in your \$PATH"
    fi

    echo "testing boskosctl"
    boskosctl --help

    # Aquire a project from boskos pool, test will use secret created on this
    if [ -n "${BOSKOS_HOST:-}" ]; then
        echo "Boskos acquire - ${BOSKOS_HOST}"
        export BOSKOS_RESOURCE="$( boskosctlwrapper acquire --type gce-project --state free --target-state busy --timeout 1h )"
        export RESOURCE_NAME=$(echo $BOSKOS_RESOURCE | jq  -r ".name")
        export GCP_PROJECT=$(echo $BOSKOS_RESOURCE | jq  -r ".name")

        # send a heartbeat in the background to keep the lease while using the resource
        echo "Starting Boskos HeartBeat"
        boskosctlwrapper heartbeat --resource "${BOSKOS_RESOURCE}" &
    fi

    echo "Using project ${GCP_PROJECT}"
    gcloud config set project ${GCP_PROJECT}

    # create a secret in the aquired project 
    export SECRET_ID="test-secret-$(openssl rand -hex 4)"
    export SECRET_VALUE="secret-a"
    echo -n ${SECRET_VALUE} | gcloud beta secrets create ${SECRET_ID} --data-file=- --ttl=1800s --quiet

    export SECRET_PROJECT_ID="$(gcloud config get project)"
    export SECRET_PROJECT_NUMBER="$(gcloud projects describe $SECRET_PROJECT_ID --format='value(projectNumber)')"

    export SECRET_URI="projects/${CLUSTER_PROJECT_NUMBER}/secrets/${SECRET_ID}/versions/latest"

    # Prow jobs are executed by `k8s-infra-prow-build.svc.id.goog` in test-pods namespace, so grant the access to the secret
    gcloud secrets add-iam-policy-binding ${SECRET_ID} \
    --role=roles/secretmanager.secretAccessor \
    --member=principalSet://iam.googleapis.com/projects/773781448124/locations/global/workloadIdentityPools/k8s-infra-prow-build.svc.id.goog/namespace/test-pods

    # wait for permissions to propogate
    sleep 60

    make e2e-bootstrap e2e-helm-deploy e2e-gcp
}

main
