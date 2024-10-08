#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests/azure
WAIT_TIME=60
SLEEP_TIME=1
NAMESPACE=default
PROVIDER_NAMESPACE=kube-system
NODE_SELECTOR_OS=linux
BASE64_FLAGS="-w 0"
if [[ "$OSTYPE" == *"darwin"* ]]; then
  BASE64_FLAGS="-b 0"
fi

if [ $TEST_WINDOWS ]; then
  NODE_SELECTOR_OS=windows
fi

export KEYVAULT_NAME=${KEYVAULT_NAME:-secrets-store-csi-e2e}
export SECRET_NAME=${KEYVAULT_SECRET_NAME:-secret1}
export SECRET_VERSION=${KEYVAULT_SECRET_VERSION:-""}
export SECRET_VALUE=${KEYVAULT_SECRET_VALUE:-"test"}
export LABEL_VALUE=${LABEL_VALUE:-"test"}
export NODE_SELECTOR_OS=$NODE_SELECTOR_OS

# export the secrets-store API version to be used
# TODO (aramase) remove this once the upgrade tests are moved to use e2e-provider
export API_VERSION=$(get_secrets_store_api_version)

setup() {
  if [[ -z "${IDENTITY_CLIENT_ID}" ]]; then
    echo "Error: Azure managed identity id is not provided" >&2
    return 1
  fi
}

@test "install azure provider" {
  # install the azure provider using the helm charts
  helm repo add csi-provider-azure https://azure.github.io/secrets-store-csi-driver-provider-azure/charts
  helm repo update
  helm upgrade --install csi csi-provider-azure/csi-secrets-store-provider-azure --namespace $PROVIDER_NAMESPACE \
        --set "secrets-store-csi-driver.install=false" \
        --set "windows.enabled=$TEST_WINDOWS" \
        --set "logVerbosity=5" \
        --set "logFormatJSON=true" \

  # wait for azure-csi-provider pod to be running
  kubectl wait --for=condition=Ready --timeout=150s pods -l app=csi-secrets-store-provider-azure --namespace $PROVIDER_NAMESPACE
}

@test "deploy azure secretproviderclass crd" {
  envsubst < $BATS_TESTS_DIR/azure_v1_secretproviderclass.yaml | kubectl apply -n $NAMESPACE -f -

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure -n $NAMESPACE -o yaml | grep azure"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
  envsubst < $BATS_TESTS_DIR/pod-secrets-store-inline-volume-crd.yaml | kubectl apply -n $NAMESPACE -f -
  
  # The wait timeout is set to 300s only for this first pod in test to accomadate for the node-driver-registrar
  # registration retries on windows nodes. Based on previous tests on windows nodes, the node-driver-registrar was
  # restarted 5 times before succeeding which resulted in a wait timeout of 300s.
  kubectl wait --for=condition=Ready --timeout=300s -n $NAMESPACE pod/secrets-store-inline-crd

  run kubectl get pod/secrets-store-inline-crd -n $NAMESPACE
  assert_success
}

@test "CSI inline volume test with pod portability - read azure kv secret from pod" {
  wait_for_process $WAIT_TIME $SLEEP_TIME "kubectl exec secrets-store-inline-crd -n $NAMESPACE -- cat /mnt/secrets-store/$SECRET_NAME | grep '${SECRET_VALUE}'"

  result=$(kubectl exec secrets-store-inline-crd -n $NAMESPACE -- cat /mnt/secrets-store/$SECRET_NAME)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]
}

@test "CSI inline volume test with pod portability - unmount succeeds" {
  # On Linux a failure to unmount the tmpfs will block the pod from being
  # deleted.
  run kubectl delete pod secrets-store-inline-crd -n $NAMESPACE
  assert_success

  run kubectl wait --for=delete --timeout=${WAIT_TIME}s pod/secrets-store-inline-crd -n $NAMESPACE
  assert_success

  # Sleep to allow time for logs to propagate.
  sleep 10

  # save debug information to archive in case of failure
  archive_info

  # On Windows, the failed unmount calls from: https://github.com/kubernetes-sigs/secrets-store-csi-driver/pull/545
  # do not prevent the pod from being deleted. Search through the driver logs
  # for the error.
  run bash -c "kubectl logs -l app=secrets-store-csi-driver --tail -1 -c secrets-store -n kube-system | grep '^E.*failed to clean and unmount target path.*$'"
  assert_failure
}

@test "Sync with K8s secrets - create deployment" {
  envsubst < $BATS_TESTS_DIR/azure_synck8s_v1_secretproviderclass.yaml | kubectl apply -n $NAMESPACE -f - 

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure-sync -n $NAMESPACE -o yaml | grep azure"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  envsubst < $BATS_TESTS_DIR/deployment-synck8s-azure.yaml | kubectl apply -n $NAMESPACE -f -
  envsubst < $BATS_TESTS_DIR/deployment-two-synck8s-azure.yaml | kubectl apply -n $NAMESPACE -f -

  kubectl wait --for=condition=Ready --timeout=90s -n $NAMESPACE pod -l app=busybox
}

@test "Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences with multiple owners" {
  POD=$(kubectl get pod -l app=busybox -n $NAMESPACE -o jsonpath="{.items[0].metadata.name}")

  result=$(kubectl exec $POD -n $NAMESPACE -- cat /mnt/secrets-store/secretalias)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl get secret foosecret -n $NAMESPACE -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec $POD -n $NAMESPACE -- printenv | grep SECRET_USERNAME) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl get secret foosecret -n $NAMESPACE -o jsonpath="{.metadata.labels.environment}")
  [[ "${result//$'\r'}" == "${LABEL_VALUE}" ]]

  result=$(kubectl get secret foosecret -n $NAMESPACE -o jsonpath="{.metadata.labels.secrets-store\.csi\.k8s\.io/managed}")
  [[ "${result//$'\r'}" == "true" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret default 2"
  assert_success
}

@test "Sync with K8s secrets - delete deployment, check owner ref updated, check secret deleted" {
  run kubectl delete -n $NAMESPACE -f $BATS_TESTS_DIR/deployment-synck8s-azure.yaml
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret default 1"
  assert_success

  run kubectl delete -n $NAMESPACE -f $BATS_TESTS_DIR/deployment-two-synck8s-azure.yaml
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted foosecret default"
  assert_success

  envsubst < $BATS_TESTS_DIR/azure_synck8s_v1_secretproviderclass.yaml | kubectl delete -n $NAMESPACE -f -
}

@test "Test Namespaced scope SecretProviderClass - create deployment" {
  run kubectl create ns test-ns
  assert_success

  envsubst < $BATS_TESTS_DIR/azure_v1_secretproviderclass_ns.yaml | kubectl apply -f -

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure-sync -n $NAMESPACE -o yaml | grep azure"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure-sync -n test-ns -o yaml | grep azure"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  envsubst < $BATS_TESTS_DIR/deployment-synck8s-azure.yaml | kubectl apply -n test-ns -f -

  kubectl wait --for=condition=Ready --timeout=60s pod -l app=busybox -n test-ns
}

@test "Test Namespaced scope SecretProviderClass - Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences" {
  POD=$(kubectl get pod -l app=busybox -n test-ns -o jsonpath="{.items[0].metadata.name}")

  result=$(kubectl exec -n test-ns $POD -- cat /mnt/secrets-store/secretalias)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl get secret foosecret -n test-ns -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec -n test-ns $POD -- printenv | grep SECRET_USERNAME) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret test-ns 1"
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - Sync with K8s secrets - delete deployment, check secret deleted" {
  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s-azure.yaml -n test-ns
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted foosecret test-ns"
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - Should fail when no secret provider class in same namespace" {
  run kubectl create ns negative-test-ns
  assert_success

  envsubst < $BATS_TESTS_DIR/deployment-synck8s-azure.yaml | kubectl apply -n negative-test-ns -f -
  sleep 5

  POD=$(kubectl get pod -l app=busybox -n negative-test-ns -o jsonpath="{.items[0].metadata.name}")
  cmd="kubectl describe pod $POD -n negative-test-ns | grep 'FailedMount.*failed to get secretproviderclass negative-test-ns/azure-sync.*not found'"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s-azure.yaml -n negative-test-ns
  assert_success

  run kubectl delete ns negative-test-ns
  assert_success
}

@test "deploy multiple azure secretproviderclass crd" {
  envsubst < $BATS_TESTS_DIR/azure_v1_multiple_secretproviderclass.yaml | kubectl apply -n $NAMESPACE -f -

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure-spc-0 -n $NAMESPACE -o yaml | grep azure-spc-0"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure-spc-1 -n $NAMESPACE -o yaml | grep azure-spc-1"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "deploy pod with multiple secret provider class" {
  envsubst < $BATS_TESTS_DIR/pod-azure-inline-volume-multiple-spc.yaml | kubectl apply -n $NAMESPACE -f -
  
  kubectl wait --for=condition=Ready --timeout=60s pod/secrets-store-inline-multiple-crd -n $NAMESPACE

  run kubectl get pod/secrets-store-inline-multiple-crd -n $NAMESPACE
  assert_success
}

@test "CSI inline volume test with multiple secret provider class" {
  result=$(kubectl exec secrets-store-inline-multiple-crd -n $NAMESPACE -- cat /mnt/secrets-store-0/secretalias)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -n $NAMESPACE -- cat /mnt/secrets-store-1/secretalias)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl get secret foosecret-0 -n $NAMESPACE -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -n $NAMESPACE -- printenv | grep SECRET_USERNAME_0) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret-0 default 1"
  assert_success

  result=$(kubectl get secret foosecret-1 -n $NAMESPACE -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -n $NAMESPACE -- printenv | grep SECRET_USERNAME_1) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret-1 default 1"
  assert_success
}

teardown_file() {
  archive_provider "app=csi-secrets-store-provider-azure" || true
  archive_info || true

  #cleanup
  run kubectl delete namespace test-ns
  run kubectl delete pods secrets-store-inline-crd secrets-store-inline-multiple-crd --force --grace-period 0
}
