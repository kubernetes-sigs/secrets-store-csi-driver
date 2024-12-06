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

main() {
    echo "starting the script"

    if [[ -z "$GOOGLE_APPLICATION_CREDENTIALS" ]]; then
        echo "GOOGLE_APPLICATION_CREDENTIALS is not set. Please set this to the path of the service account used to run this script."
    else
        gcloud auth activate-service-account --key-file="${GOOGLE_APPLICATION_CREDENTIALS}"
    fi
    GCP_PROJECT=$(jq -r .project_id "${GOOGLE_APPLICATION_CREDENTIALS}")
    echo "Using project ${GCP_PROJECT}"

    # make e2e-bootstrap e2e-helm-deploy e2e-gcp

}

main