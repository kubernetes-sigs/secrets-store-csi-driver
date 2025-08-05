#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests/e2e_provider
WAIT_TIME=60
SLEEP_TIME=1
NODE_SELECTOR_OS=linux
BASE64_FLAGS="-w 0"
if [[ "$OSTYPE" == *"darwin"* ]]; then
  BASE64_FLAGS="-b 0"
fi

# export secret vars
export SECRET_NAME=${SECRET_NAME:-foo}
# defualt version value returned by mock provider
export SECRET_VERSION=${SECRET_VERSION:-"v1"}
# default secret value returned by the mock provider
export SECRET_VALUE=${SECRET_VALUE:-"secret"}
# default secret mode returned by the mock provider
export SECRET_MODE=${SECRET_MODE:-'"0644"'}

# export key vars
export KEY_NAME=${KEY_NAME:-fookey}
# defualt version value returned by mock provider
export KEY_VERSION=${KEY_VERSION:-"v1"}
# default key value returned by mock provider.
# base64 encoded content comparision is easier in case of very long multiline string.
export KEY_VALUE_CONTAINS=${KEY_VALUE:-"LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KVGhpcyBpcyBtb2NrIGtleQotLS0tLUVORCBQVUJMSUMgS0VZLS0tLS0K"}
# defualt version value returned by mock provider
export KEY_MODE=${KEY_MODE:-'"0644"'}

# export node selector var
export NODE_SELECTOR_OS=$NODE_SELECTOR_OS
# default label value of secret synched to k8s
export LABEL_VALUE=${LABEL_VALUE:-"test"}

# export the secrets-store API version to be used
export API_VERSION=$(get_secrets_store_api_version)
export NAMESPACE=${NAMESPACE:-"default"}
export SPC_NAME=${SPC_NAME:-"e2e-provider"}
export POD_NAME=${POD_NAME:-"secrets-store-inline-crd"}
export POD_SECURITY_CONTEXT=${POD_SECURITY_CONTEXT:-""}
export CONTAINER_SECURITY_CONTEXT=${CONTAINER_SECURITY_CONTEXT:-""}

# export the token requests audience configured in the CSIDriver
# refer to https://kubernetes-csi.github.io/docs/token-requests.html for more information
export VALIDATE_TOKENS_AUDIENCE=$(get_token_requests_audience)

#########################################################
# begin: Utility functions to perform common operations #
#########################################################
function create_spc() {
  envsubst < $BATS_TESTS_DIR/e2e_provider_secretproviderclass.yaml | kubectl -n $NAMESPACE apply -f -

  kubectl -n $NAMESPACE wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl -n $NAMESPACE get secretproviderclasses.secrets-store.csi.x-k8s.io/$SPC_NAME -o yaml | grep $SPC_NAME"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

function create_pod() {
  envsubst < $BATS_TESTS_DIR/pod-secrets-store-inline-volume-crd.yaml | kubectl -n $NAMESPACE apply -f -
  kubectl -n $NAMESPACE wait --for=condition=Ready --timeout=180s pod/$POD_NAME
  run kubectl -n $NAMESPACE get pod/$POD_NAME
  assert_success
}

# POD_NAME could have come as a parameter in the below 3 functions.
# But keeping the semantics uniform with other functions where envsubst is used
function read_secret() {
  wait_for_process $WAIT_TIME $SLEEP_TIME "kubectl -n $NAMESPACE exec $POD_NAME -- ls /mnt/secrets-store/$SECRET_NAME"
  local result=$(kubectl -n $NAMESPACE exec $POD_NAME -- cat /mnt/secrets-store/$SECRET_NAME)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]
}

function read_key() {
  result=$(kubectl -n $NAMESPACE exec $POD_NAME -- cat /mnt/secrets-store/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]
}

function delete_pod() {
  # On Linux a failure to unmount the tmpfs will block the pod from being
  # deleted.
  run kubectl -n $NAMESPACE delete pod $POD_NAME
  assert_success

  kubectl -n $NAMESPACE wait --for=delete --timeout=${WAIT_TIME}s pod/$POD_NAME
  assert_success

  # Sleep to allow time for logs to propagate.
  sleep 10

  # save debug information to archive in case of failure
  archive_info

  # On Windows, the failed unmount calls from: https://github.com/kubernetes-sigs/secrets-store-csi-driver/pull/545
  # do not prevent the pod from being deleted. Search through the driver logs
  # for the error.
  run bash -c "kubectl -n $NAMESPACE logs -l app=$POD_NAME --tail -1 -c secrets-store -n kube-system | grep '^E.*failed to clean and unmount target path.*$'"
  assert_failure
}

function enable_secret_rotation() {
  # enable rotation response in mock server
  local curl_pod_name=curl-$(openssl rand -hex 5)
  kubectl run ${curl_pod_name} -n rotation --image=curlimages/curl:7.75.0 --labels="test=rotation" -- tail -f /dev/null
  kubectl wait -n rotation --for=condition=Ready --timeout=60s pod ${curl_pod_name}
  local pod_ip=$(kubectl get pod -n kube-system -l app=csi-secrets-store-e2e-provider -o jsonpath="{.items[0].status.podIP}")
  run kubectl exec ${curl_pod_name} -n rotation -- curl http://${pod_ip}:8080/rotation?rotated=true
  sleep 60
}

function disable_secret_rotation() {
  local curl_pod_name=$1
  local pod_ip=$(kubectl get pod -n kube-system -l app=csi-secrets-store-e2e-provider -o jsonpath="{.items[0].status.podIP}")  
  run kubectl exec ${curl_pod_name} -n rotation -- curl http://${pod_ip}:8080/rotation?rotated=false
}
#######################################################
# end: Utility functions to perform common operations #
#######################################################

@test "setup mock provider validation config" {
  if [[ -n "${VALIDATE_TOKENS_AUDIENCE}" ]]; then
    # configure the mock provider to validate the token requests
    kubectl create ns enable-token-requests
    local curl_pod_name=curl-$(openssl rand -hex 5)
    kubectl run ${curl_pod_name} -n enable-token-requests --image=curlimages/curl:7.75.0 --labels="util=enable-token-requests" -- tail -f /dev/null
    kubectl wait -n enable-token-requests --for=condition=Ready --timeout=60s pod ${curl_pod_name}
    local pod_ip=$(kubectl get pod -n kube-system -l app=csi-secrets-store-e2e-provider -o jsonpath="{.items[0].status.podIP}")
    run kubectl exec ${curl_pod_name} -n enable-token-requests -- curl http://${pod_ip}:8080/validate-token-requests?audience=${VALIDATE_TOKENS_AUDIENCE}
    kubectl delete pod -l util=enable-token-requests -n enable-token-requests --force --grace-period 0
    kubectl delete ns enable-token-requests
  fi

  log_secrets_store_api_version
  log_token_requests_audience
}


@test "secretproviderclasses crd is established" {
  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  run kubectl get crd/secretproviderclasses.secrets-store.csi.x-k8s.io
  assert_success
}

@test "secretproviderclasspodstatuses crd is established" {
  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasspodstatuses.secrets-store.csi.x-k8s.io

  run kubectl get crd/secretproviderclasspodstatuses.secrets-store.csi.x-k8s.io
  assert_success
}

@test "Test rbac roles and role bindings exist" {
  run kubectl get clusterrole/secretproviderclasses-role
  assert_success

  run kubectl get clusterrole/secretproviderclasses-admin-role
  assert_success

  run kubectl get clusterrole/secretproviderclasses-viewer-role
  assert_success

  run kubectl get clusterrole/secretproviderclasspodstatuses-viewer-role
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

  # validate token request role and rolebinding only when token requests are set
  if [[ -n "${VALIDATE_TOKENS_AUDIENCE}" ]]; then
    run kubectl get clusterrole/secretprovidertokenrequest-role
    assert_success

    run kubectl get clusterrolebinding/secretprovidertokenrequest-rolebinding
    assert_success
  fi
}

@test "[v1alpha1] deploy e2e-provider secretproviderclass crd" {
  kubectl create namespace test-v1alpha1 --dry-run=client -o yaml | kubectl apply -f -
  NAMESPACE="test-v1alpha1" API_VERSION="secrets-store.csi.x-k8s.io/v1alpha1" create_spc
}

@test "[v1alpha1] CSI inline volume test with pod portability" {
  NAMESPACE="test-v1alpha1" create_pod
}

@test "[v1alpha1] CSI inline volume test with pod portability - read secret from pod" {
  NAMESPACE="test-v1alpha1" read_secret
}

@test "[v1alpha1] CSI inline volume test with pod portability - read key from pod" {
  NAMESPACE="test-v1alpha1" read_key  
}

@test "[v1alpha1] CSI inline volume test with pod portability - unmount succeeds" {
  NAMESPACE="test-v1alpha1" delete_pod
}

@test "deploy e2e-provider v1 secretproviderclass crd" {
  create_spc
}

@test "CSI inline volume test with pod portability" {
  create_pod
}

@test "CSI inline volume test with pod portability - read secret from pod" {
  read_secret
}

@test "CSI inline volume test with pod portability - read key from pod" {
  read_key
}

@test "CSI inline volume test with pod portability - unmount succeeds" {
  delete_pod
}

@test "deploy e2e-provider v1 secretproviderclass crd with restricted permissions" {
  SPC_NAME="e2e-provider-640" SECRET_MODE='"0640"' KEY_MODE='"0640"' create_spc
}

@test "Non-root POD with no FSGroup - create" {
  SPC_NAME="e2e-provider-640" POD_NAME="non-root-with-no-fsgroup" POD_SECURITY_CONTEXT='"runAsNonRoot": true, "runAsUser": 1004, "runAsGroup": 1004' create_pod
}

@test "Non-root POD with no FSGroup - Should fail to read non world readable secret" {
  # use run here as read_secret will run into errors and we want to assert_failure
  POD_NAME="non-root-with-no-fsgroup" run read_secret
  assert_failure
}

@test "Non-root POD with no FSGroup - unmount succeeds" {
   POD_NAME="non-root-with-no-fsgroup" delete_pod
}

@test "Non-root POD with FSGroup - create" {
  SPC_NAME="e2e-provider-640" POD_NAME="non-root-with-fsgroup" POD_SECURITY_CONTEXT='"runAsNonRoot": true, "runAsUser": 1004, "runAsGroup": 1004, "fsGroup": 1004' create_pod
}

@test "Non-root POD with FSGroup - should read non world readable secret" {
  POD_NAME="non-root-with-fsgroup" read_secret
}

@test "Non-root POD with FSGroup - rotated secret should also be readable" {
  curl_pod_name=$(enable_secret_rotation)
  SECRET_VALUE="rotated" POD_NAME="non-root-with-fsgroup" read_secret
  disable_secret_rotation $curl_pod_name
}

@test "Non-root POD with FSGroup - unmount succeeds" {
   POD_NAME="non-root-with-fsgroup" delete_pod
}

@test "Sync with K8s secrets - create deployment" {
  envsubst < $BATS_TESTS_DIR/e2e_provider_synck8s_v1_secretproviderclass.yaml | kubectl apply -f -

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-provider-sync -o yaml | grep e2e-provider"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  envsubst < $BATS_TESTS_DIR/deployment-synck8s-e2e-provider.yaml | kubectl apply -f -
  envsubst < $BATS_TESTS_DIR/deployment-two-synck8s-e2e-provider.yaml | kubectl apply -f -

  kubectl wait --for=condition=Ready --timeout=90s pod -l app=busybox
}

@test "Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences with multiple owners" {
  POD=$(kubectl get pod -l app=busybox -o jsonpath="{.items[0].metadata.name}")

  result=$(kubectl exec $POD -- cat /mnt/secrets-store/$SECRET_NAME)
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
  if [[ "${INPLACE_UPGRADE_TEST}" == "true" ]]; then
    skip
  fi

  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s-e2e-provider.yaml
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret default 1"
  assert_success

  run kubectl delete -f $BATS_TESTS_DIR/deployment-two-synck8s-e2e-provider.yaml
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted foosecret default"
  assert_success

  envsubst < $BATS_TESTS_DIR/e2e_provider_synck8s_v1_secretproviderclass.yaml | kubectl delete -f -
}

@test "Test Namespaced scope SecretProviderClass - create deployment" {
  kubectl create namespace test-ns --dry-run=client -o yaml | kubectl apply -f -

  envsubst < $BATS_TESTS_DIR/e2e_provider_v1_secretproviderclass_ns.yaml | kubectl apply -f -

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-provider-sync -o yaml | grep e2e-provider"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-provider-sync -n test-ns -o yaml | grep e2e-provider"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  envsubst < $BATS_TESTS_DIR/deployment-synck8s-e2e-provider.yaml | kubectl apply -n test-ns -f -

  kubectl wait --for=condition=Ready --timeout=60s pod -l app=busybox -n test-ns
}

@test "Test Namespaced scope SecretProviderClass - Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences" {
  POD=$(kubectl get pod -l app=busybox -n test-ns -o jsonpath="{.items[0].metadata.name}")

  result=$(kubectl exec -n test-ns $POD -- cat /mnt/secrets-store/$SECRET_NAME)
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
  if [[ "${INPLACE_UPGRADE_TEST}" == "true" ]]; then
    skip
  fi
  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s-e2e-provider.yaml -n test-ns
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted foosecret test-ns"
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - Should fail when no secret provider class in same namespace" {
  kubectl create namespace negative-test-ns --dry-run=client -o yaml | kubectl apply -f -

  envsubst < $BATS_TESTS_DIR/deployment-synck8s-e2e-provider.yaml | kubectl apply -n negative-test-ns -f -
  sleep 5

  POD=$(kubectl get pod -l app=busybox -n negative-test-ns -o jsonpath="{.items[0].metadata.name}")
  cmd="kubectl describe pod $POD -n negative-test-ns | grep 'FailedMount.*failed to get secretproviderclass negative-test-ns/e2e-provider-sync.*not found'"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s-e2e-provider.yaml -n negative-test-ns
  assert_success

  if [[ "${INPLACE_UPGRADE_TEST}" != "true" ]]; then
    run kubectl delete ns negative-test-ns
    assert_success
  fi
}

@test "deploy multiple e2e provier secretproviderclass crd" {
  envsubst < $BATS_TESTS_DIR/e2e_provider_v1_multiple_secretproviderclass.yaml | kubectl apply -f -

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-provider-spc-0 -o yaml | grep e2e-provider-spc-0"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-provider-spc-1 -o yaml | grep e2e-provider-spc-1"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "deploy pod with multiple secret provider class" {
  envsubst < $BATS_TESTS_DIR/pod-e2e-provider-inline-volume-multiple-spc.yaml | kubectl apply -f -

  kubectl wait --for=condition=Ready --timeout=60s pod/secrets-store-inline-multiple-crd

  run kubectl get pod/secrets-store-inline-multiple-crd
  assert_success
}

@test "CSI inline volume test with multiple secret provider class" {
  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-0/$SECRET_NAME)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-0/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-1/$SECRET_NAME)
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

@test "Test auto rotation of mount contents and K8s secrets - Create deployment" {

  envsubst < $BATS_TESTS_DIR/rotation/e2e_provider_synck8s_v1_secretproviderclass.yaml | kubectl apply -n rotation -f -
  envsubst < $BATS_TESTS_DIR/rotation/pod-synck8s-e2e-provider.yaml | kubectl apply -n rotation -f -

  kubectl wait -n rotation --for=condition=Ready --timeout=60s pod/secrets-store-inline-rotation

  run kubectl get pod/secrets-store-inline-rotation -n rotation
  assert_success
}

@test "Test auto rotation of mount contents and K8s secrets" {
  result=$(kubectl exec -n rotation secrets-store-inline-rotation -- cat /mnt/secrets-store/$SECRET_NAME)
  [[ "${result//$'\r'}" == "secret" ]]

  result=$(kubectl get secret -n rotation rotationsecret -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "secret" ]]

  curl_pod_name=$(enable_secret_rotation)


  result=$(kubectl exec -n rotation secrets-store-inline-rotation -- cat /mnt/secrets-store/$SECRET_NAME)
  [[ "${result//$'\r'}" == "rotated" ]]

  result=$(kubectl get secret -n rotation rotationsecret -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" == "rotated" ]]

  # reset rotation response in mock server for all upgrade tests
  disable_secret_rotation $curl_pod_name

}

@test "Validate metrics" {
  kubectl create ns metrics
  local curl_pod_name=curl-$(openssl rand -hex 5)
  kubectl run ${curl_pod_name} -n metrics --image=curlimages/curl:7.75.0 --labels="test=metrics" -- tail -f /dev/null
  kubectl wait -n metrics --for=condition=Ready --timeout=60s pod ${curl_pod_name}
  for pod_ip in $(kubectl get pod -n kube-system -l app=secrets-store-csi-driver -o jsonpath="{.items[0].status.podIP}")
  do
    run kubectl exec ${curl_pod_name} -n metrics -- curl http://${pod_ip}:8095/metrics
    assert_match "node_publish_total" "${output}"
    assert_match "node_unpublish_total" "${output}"
    assert_match "rotation_reconcile_total" "${output}"
  done
  # keeping metrics ns in upgrade tests has no relevance
  # the namespace is only to run a curl pod to validate metrics
  # so it should be fine to delete and recreate it during upgrade tests
  kubectl delete ns metrics
}

setup_file() {
    kubectl create namespace rotation --dry-run=client -o yaml | kubectl apply -f -
}

teardown_file() {
  if [[ "${INPLACE_UPGRADE_TEST}" != "true" ]]; then
    #cleanup
    run kubectl delete namespace rotation
    run kubectl delete namespace test-ns
    run kubectl delete namespace test-v1alpha1

    run kubectl delete pods secrets-store-inline-crd secrets-store-inline-multiple-crd --force --grace-period 0
  fi
}
