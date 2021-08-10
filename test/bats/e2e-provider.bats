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
  skip
  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  run kubectl get crd/secretproviderclasses.secrets-store.csi.x-k8s.io
  assert_success
}

@test "Test rbac roles and role bindings exist" {
  skip
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

@test "deploy e2e-provider secretproviderclass crd" {
  # skip
  envsubst < $BATS_TESTS_DIR/e2e_provider_v1alpha1_secretproviderclass.yaml | kubectl apply -f -

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-provider -o yaml | grep e2e-provider"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
  # skip
  envsubst < $BATS_TESTS_DIR/pod-secrets-store-inline-volume-crd.yaml | kubectl apply -f -
  
  kubectl wait --for=condition=Ready --timeout=180s pod/secrets-store-inline-crd

  run kubectl get pod/secrets-store-inline-crd
  assert_success
}

@test "CSI inline volume test with pod portability - read kv secret from pod" {
  skip
  wait_for_process $WAIT_TIME $SLEEP_TIME "kubectl exec secrets-store-inline-crd -- cat /mnt/secrets-store/$SECRET_NAME | grep '${SECRET_VALUE}'"

  result=$(kubectl exec secrets-store-inline-crd -- cat /mnt/secrets-store/$SECRET_NAME)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]
}

@test "CSI inline volume test with pod portability - read kv key from pod" {
  skip
  result=$(kubectl exec secrets-store-inline-crd -- cat /mnt/secrets-store/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]
}

@test "CSI inline volume test with pod portability - unmount succeeds" {
  skip
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

@test "Sync with K8s secrets - create deployment" {
  skip
  envsubst < $BATS_TESTS_DIR/e2e_provider_synck8s_v1alpha1_secretproviderclass.yaml | kubectl apply -f - 

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-provider-sync -o yaml | grep e2e-provider"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  envsubst < $BATS_TESTS_DIR/deployment-synck8s-e2e-provider.yaml | kubectl apply -f -
  envsubst < $BATS_TESTS_DIR/deployment-two-synck8s-e2e-provider.yaml | kubectl apply -f -

  kubectl wait --for=condition=Ready --timeout=90s pod -l app=busybox
}

@test "Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences with multiple owners" {
  skip
  POD=$(kubectl get pod -l app=busybox -o jsonpath="{.items[0].metadata.name}")

  result=$(kubectl exec $POD -- cat /mnt/secrets-store/secretalias)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec $POD -- cat /mnt/secrets-store/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec $POD -- printenv | grep SECRET_USERNAME) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.metadata.labels.environment}")
  [[ "${result//$'\r'}" == "${LABEL_VALUE}" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.metadata.labels.secrets-store\.csi\.k8s\.io/managed}")
  [[ "${result//$'\r'}" == "true" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret default 2"
  assert_success
}

@test "Sync with K8s secrets - delete deployment, check owner ref updated, check secret deleted" {
  skip
  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s-e2e-provider.yaml
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret default 1"
  assert_success

  run kubectl delete -f $BATS_TESTS_DIR/deployment-two-synck8s-e2e-provider.yaml
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted foosecret default"
  assert_success

  run kubectl delete -f $BATS_TESTS_DIR/e2e_provider_synck8s_v1alpha1_secretproviderclass.yaml
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - create deployment" {
  skip
  run kubectl create ns test-ns
  assert_success

  envsubst < $BATS_TESTS_DIR/e2e_provider_v1alpha1_secretproviderclass_ns.yaml | kubectl apply -f -

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-provider-sync -o yaml | grep e2e-provider"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-provider-sync -n test-ns -o yaml | grep e2e-provider"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  envsubst < $BATS_TESTS_DIR/deployment-synck8s-e2e-provider.yaml | kubectl apply -n test-ns -f -

  kubectl wait --for=condition=Ready --timeout=60s pod -l app=busybox -n test-ns
}

@test "Test Namespaced scope SecretProviderClass - Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences" {
  skip
  POD=$(kubectl get pod -l app=busybox -n test-ns -o jsonpath="{.items[0].metadata.name}")

  result=$(kubectl exec -n test-ns $POD -- cat /mnt/secrets-store/secretalias)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec -n test-ns $POD -- cat /mnt/secrets-store/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]

  result=$(kubectl get secret foosecret -n test-ns -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec -n test-ns $POD -- printenv | grep SECRET_USERNAME) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret test-ns 1"
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - Sync with K8s secrets - delete deployment, check secret deleted" {
  skip
  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s-e2e-provider.yaml -n test-ns
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted foosecret test-ns"
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - Should fail when no secret provider class in same namespace" {
  skip
  run kubectl create ns negative-test-ns
  assert_success

  envsubst < $BATS_TESTS_DIR/deployment-synck8s-e2e-provider.yaml | kubectl apply -n negative-test-ns -f -
  sleep 5

  POD=$(kubectl get pod -l app=busybox -n negative-test-ns -o jsonpath="{.items[0].metadata.name}")
  cmd="kubectl describe pod $POD -n negative-test-ns | grep 'FailedMount.*failed to get secretproviderclass negative-test-ns/e2e-provider-sync.*not found'"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s-e2e-provider.yaml -n negative-test-ns
  assert_success

  run kubectl delete ns negative-test-ns
  assert_success
}

@test "deploy multiple e2e provier secretproviderclass crd" {
  skip
  envsubst < $BATS_TESTS_DIR/e2e-provider_v1alpha1_multiple_secretproviderclass.yaml | kubectl apply -f -

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-provider-spc-0 -o yaml | grep e2e-provider-spc-0"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-provider-spc-1 -o yaml | grep e2e-provider-spc-1"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "deploy pod with multiple secret provider class" {
  skip
  envsubst < $BATS_TESTS_DIR/pod-e2e-provider-inline-volume-multiple-spc.yaml | kubectl apply -f -
  
  kubectl wait --for=condition=Ready --timeout=60s pod/secrets-store-inline-multiple-crd

  run kubectl get pod/secrets-store-inline-multiple-crd
  assert_success
}

@test "CSI inline volume test with multiple secret provider class" {
  skip
  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-0/secretalias)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-0/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-1/secretalias)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-1/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]

  result=$(kubectl get secret foosecret-0 -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- printenv | grep SECRET_USERNAME_0) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret-0 default 1"
  assert_success

  result=$(kubectl get secret foosecret-1 -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- printenv | grep SECRET_USERNAME_1) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret-1 default 1"
  assert_success
}
