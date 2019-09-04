#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests
WAIT_TIME=60
SLEEP_TIME=1
IMAGE_TAG=e2e-$(git rev-parse --short HEAD)

export KEYVAULT_NAME=secrets-store-csi-e2e
export RESOURCE_GROUP=secrets-store-csi-driver-e2e
export SUBSCRIPTION_ID=940f88ce-a64b-4e73-a258-9931349b9789
export TENANT_ID=microsoft.com
export SECRET_NAME=secret1
export KEY_NAME=key1
export KEY_VERSION=06baad80c1f74e51868fd2271ef2b06c
export SECRET_NAME=secret1
export SECRET_VERSION=""

setup() {
  if [[ -z "${AZURE_CLIENT_ID}" ]] || [[ -z "${AZURE_CLIENT_SECRET}" ]]; then
    echo "Error: Azure service principal is not provided" >&2
    return 1
  fi
}

@test "install helm chart with e2e image" {
  run helm install charts/secrets-store-csi-driver -n csi-secrets-store --namespace dev \
          --set image.pullPolicy="IfNotPresent" \
          --set image.repository="e2e/secrets-store-csi" \
          --set image.tag=$IMAGE_TAG
  assert_success
}

@test "csi-secrets-store-attacher-0 is running" {
  cmd="kubectl wait -n dev --for=condition=Ready --timeout=60s pod/csi-secrets-store-attacher-0"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/csi-secrets-store-attacher-0 -n dev
  assert_success
}

@test "create azure k8s secret" {
  run kubectl create secret generic secrets-store-creds --from-literal clientid=${AZURE_CLIENT_ID} --from-literal clientsecret=${AZURE_CLIENT_SECRET}
  assert_success
}

@test "CSI inline volume test" {
  envsubst < $BATS_TESTS_DIR/nginx-pod-secrets-store-inline-volume.yaml | kubectl apply -f -

  cmd="kubectl wait --for=condition=Ready --timeout=60s pod/nginx-secrets-store-inline"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/nginx-secrets-store-inline
  assert_success
}

@test "CSI inline volume test - read azure kv secret from pod" {
  result=$(kubectl exec -it nginx-secrets-store-inline cat /mnt/secrets-store/secret1)
  [[ "$result" -eq "test" ]]
}

@test "CSI inline volume test - read azure kv key from pod" {
  KEY_VALUE_CONTAINS=yOtivc0OMjJ
  result=$(kubectl exec -it nginx-secrets-store-inline cat /mnt/secrets-store/key1)
  [[ "$result" == *"${KEY_VALUE_CONTAINS}"* ]]
}

@test "secretproviderclasses crd is established" {
  cmd="kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.k8s.com"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get crd/secretproviderclasses.secrets-store.csi.k8s.com
  assert_success
}

@test "deploy azure secretproviderclass crd" {
  envsubst < $BATS_TESTS_DIR/azure_v1alpha1_secretproviderclass.yaml | kubectl apply -f -

  cmd="kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.k8s.com"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.k8s.com/azure -o yaml | grep azure"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
  run kubectl apply -f $BATS_TESTS_DIR/nginx-pod-secrets-store-inline-volume-crd.yaml
  assert_success

  cmd="kubectl wait --for=condition=Ready --timeout=60s pod/nginx-secrets-store-inline-crd"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/nginx-secrets-store-inline-crd
  assert_success
}

@test "CSI inline volume test with pod portability - read azure kv secret from pod" {
  result=$(kubectl exec -it nginx-secrets-store-inline-crd cat /mnt/secrets-store/secret1)
  [[ "$result" -eq "test" ]]
}

@test "CSI inline volume test with pod portability - read azure kv key from pod" {
  KEY_VALUE_CONTAINS=yOtivc0OMjJ
  result=$(kubectl exec -it nginx-secrets-store-inline-crd cat /mnt/secrets-store/key1)
  [[ "$result" == *"${KEY_VALUE_CONTAINS}"* ]]
}
