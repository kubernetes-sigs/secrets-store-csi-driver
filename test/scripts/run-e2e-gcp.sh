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

      if [ -n "${BOSKOS_HOST:-}" ]; then
        # Check out the account from Boskos and store the produced environment
        # variables in a temporary file.
        account_env_var_file="$(mktemp)"
        python3 hack/checkout_account.py 1>"${account_env_var_file}"
        checkout_account_status="${?}"

        # If the checkout process was a success then load the account's
        # environment variables into this process.
        # shellcheck disable=SC1090
        [ "${checkout_account_status}" = "0" ] && . "${account_env_var_file}"

        # Always remove the account environment variable file. It contains
        # sensitive information.
        rm -f "${account_env_var_file}"

        if [ ! "${checkout_account_status}" = "0" ]; then
        echo "error getting account from boskos" 1>&2
        exit "${checkout_account_status}"
        fi

        # run the heart beat process to tell boskos that we are still
        # using the checked out account periodically
        python3 -u hack/heartbeat_account.py >> "$ARTIFACTS/logs/boskos.log" 2>&1 &
        # shellcheck disable=SC2116
        HEART_BEAT_PID=$(echo $!)
    fi

    if [[ -z "$GOOGLE_APPLICATION_CREDENTIALS" ]]; then
        echo "GOOGLE_APPLICATION_CREDENTIALS is not set. Please set this to the path of the service account used to run this script."
    else
        gcloud auth activate-service-account --key-file="${GOOGLE_APPLICATION_CREDENTIALS}"
    fi
    GCP_PROJECT=$(jq -r .project_id "${GOOGLE_APPLICATION_CREDENTIALS}")
    echo "Using project ${GCP_PROJECT}"

    # make e2e-bootstrap e2e-helm-deploy e2e-gcp

# If Boskos is being used then release the GCP project back to Boskos.
  [ -z "${BOSKOS_HOST:-}" ] || hack/checkin_account.py >> "$ARTIFACTS"/logs/boskos.log 2>&1
}

main