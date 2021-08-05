#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests/e2e_provider
WAIT_TIME=60
SLEEP_TIME=1
NODE_SELECTOR_OS=linux

# export secret vars
export SECRET_NAME=${KEYVAULT_SECRET_NAME:-foo}
export SECRET_VERSION=${KEYVAULT_SECRET_VERSION:-"v1"}
export SECRET_VALUE=${KEYVAULT_SECRET_VALUE:-"bar"}

# export key vars
export KEY_NAME=${KEYVAULT_KEY_NAME:-fookey}
export KEY_VERSION=${KEYVAULT_KEY_VERSION:-"v1"}
export KEY_VALUE_CONTAINS=${KEYVAULT_KEY_VALUE:-"LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KVGhpcyBpcyBmYWtlIGtleQotLS0tLUVORCBQVUJMSUMgS0VZLS0tLS0K"}

# export node selector var
export NODE_SELECTOR_OS=$NODE_SELECTOR_OS

@test "secretproviderclasses crd is established" {
  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  run kubectl get crd/secretproviderclasses.secrets-store.csi.x-k8s.io
  assert_success
}

@test "Test rbac roles and role bindings exist" {
  run kubectl get clusterrole/secretproviderclasses-role
  assert_success

  run kubectl get clusterrole/secretproviderrotation-role
  assert_success

  run kubectl get clusterrole/secretprovidersyncing-role
  assert_success

  run kubectl get clusterrolebinding/secretproviderclasses-rolebinding
  assert_success

  run kubectl get clusterrolebinding/secretproviderrotation-rolebinding
  assert_success

  run kubectl get clusterrolebinding/secretprovidersyncing-rolebinding
  assert_success
}

@test "deploy fake-provider secretproviderclass crd" {
  envsubst < $BATS_TESTS_DIR/e2e_provider_v1alpha1_secretproviderclass.yaml | kubectl apply -f -

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/fake-provider -o yaml | grep fake-provider"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
  envsubst < $BATS_TESTS_DIR/pod-secrets-store-inline-volume-crd.yaml | kubectl apply -f -
  
  kubectl wait --for=condition=Ready --timeout=180s pod/secrets-store-inline-crd

  run kubectl get pod/secrets-store-inline-crd
  assert_success
}

@test "CSI inline volume test with pod portability - read kv secret from pod" {
  wait_for_process $WAIT_TIME $SLEEP_TIME "kubectl exec secrets-store-inline-crd -- cat /mnt/secrets-store/$SECRET_NAME | grep '${SECRET_VALUE}'"

  result=$(kubectl exec secrets-store-inline-crd -- cat /mnt/secrets-store/$SECRET_NAME)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]
}

@test "CSI inline volume test with pod portability - read kv key from pod" {
  result=$(kubectl exec secrets-store-inline-crd -- cat /mnt/secrets-store/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]
}

@test "CSI inline volume test with pod portability - unmount succeeds" {
  # https://github.com/kubernetes/kubernetes/pull/96702
  # kubectl wait --for=delete does not work on already deleted pods.
  # Instead we will start the wait before initiating the delete.
  kubectl wait --for=delete --timeout=${WAIT_TIME}s pod/secrets-store-inline-crd &
  WAIT_PID=$!

  sleep 1
  run kubectl delete pod secrets-store-inline-crd

  # On Linux a failure to unmount the tmpfs will block the pod from being
  # deleted.
  run wait $WAIT_PID
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