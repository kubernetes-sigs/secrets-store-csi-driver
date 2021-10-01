#!/usr/bin/env bash

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

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[@]}")/..

TOOLS_DIR=$(realpath ./hack/tools)
TOOLS_BIN_DIR="${TOOLS_DIR}/bin"

pushd "${SCRIPT_ROOT}"
# install the generators if they are not already present
for GENERATOR in client-gen lister-gen informer-gen register-gen; do
  cd "${TOOLS_DIR}" && go build -tags=tools -o "${TOOLS_BIN_DIR}"/${GENERATOR} k8s.io/code-generator/cmd/${GENERATOR}
done
popd

OUTPUT_PKG=sigs.k8s.io/secrets-store-csi-driver/pkg/client
FQ_APIS=sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1,sigs.k8s.io/secrets-store-csi-driver/apis/v1
APIS_PKG=sigs.k8s.io/secrets-store-csi-driver
CLIENTSET_NAME=versioned
CLIENTSET_PKG_NAME=clientset

if [[ "${VERIFY_CODEGEN:-}" == "true" ]]; then
  echo "Running in verification mode"
  VERIFY_FLAG="--verify-only"
fi
COMMON_FLAGS="${VERIFY_FLAG:-}"

# reference from https://github.com/servicemeshinterface/smi-sdk-go/blob/master/hack/update-codegen.sh
# the generate-groups.sh script cannot handle group names with dashes, so we use secretsstore.csi.x-k8s.io as the group name
if [[ "$OSTYPE" == "darwin"* ]]; then
  find "${SCRIPT_ROOT}/apis" -type f -exec sed -i '' 's/secrets-store.csi.x-k8s.io/secretsstore.csi.x-k8s.io/g' {} +
else
  find "${SCRIPT_ROOT}/apis" -type f -exec sed -i 's/secrets-store.csi.x-k8s.io/secretsstore.csi.x-k8s.io/g' {} +
fi

echo "Generating clientset at ${OUTPUT_PKG}/${CLIENTSET_PKG_NAME}"
"${TOOLS_BIN_DIR}/client-gen" \
    --clientset-name "${CLIENTSET_NAME}" \
    --input-base "" \
    --input "${FQ_APIS}" \
    --output-package "${OUTPUT_PKG}/${CLIENTSET_PKG_NAME}" \
    --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${COMMON_FLAGS}"

echo "Generating listers at ${OUTPUT_PKG}/listers"
"${TOOLS_BIN_DIR}/lister-gen" \
    --input-dirs "${FQ_APIS}" \
    --output-package "${OUTPUT_PKG}/listers" \
    --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${COMMON_FLAGS}"

echo "Generating informers at ${OUTPUT_PKG}/informers"
"${TOOLS_BIN_DIR}/informer-gen" \
    --input-dirs "${FQ_APIS}" \
    --versioned-clientset-package "${OUTPUT_PKG}/${CLIENTSET_PKG_NAME}/${CLIENTSET_NAME}" \
    --listers-package "${OUTPUT_PKG}/listers" \
    --output-package "${OUTPUT_PKG}/informers" \
    --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${COMMON_FLAGS}"

for VERSION in v1alpha1 v1
do
  echo "Generating ${VERSION} register at ${APIS_PKG}/apis/${VERSION}"
  "${TOOLS_BIN_DIR}/register-gen" \
      --input-dirs "${FQ_APIS}" \
      --output-package "${APIS_PKG}/apis/${VERSION}" \
      --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
      "${COMMON_FLAGS}"

done

# reference from https://github.com/servicemeshinterface/smi-sdk-go/blob/master/hack/update-codegen.sh
# replace secretsstore.csi.x-k8s.io with secrets-store.csi.x-k8s.io after code generation
if [[ "$OSTYPE" == "darwin"* ]]; then
  find "${SCRIPT_ROOT}/apis" -type f -exec sed -i '' 's/secretsstore.csi.x-k8s.io/secrets-store.csi.x-k8s.io/g' {} +
  find "${SCRIPT_ROOT}/pkg/client" -type f -exec sed -i '' 's/secretsstore.csi.x-k8s.io/secrets-store.csi.x-k8s.io/g' {} +
else
  find "${SCRIPT_ROOT}/apis" -type f -exec sed -i 's/secretsstore.csi.x-k8s.io/secrets-store.csi.x-k8s.io/g' {} +
  find "${SCRIPT_ROOT}/pkg/client" -type f -exec sed -i 's/secretsstore.csi.x-k8s.io/secrets-store.csi.x-k8s.io/g' {} +
fi
