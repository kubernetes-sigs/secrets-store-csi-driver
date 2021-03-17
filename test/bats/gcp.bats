#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests/gcp
WAIT_TIME=60
SLEEP_TIME=1
NAMESPACE=default
PROVIDER_NAMESPACE=kube-system
PROVIDER_YAML=https://raw.githubusercontent.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/main/deploy/provider-gcp-plugin.yaml
CONTAINER_IMAGE=nginx
EXEC_COMMAND="cat"
BASE64_FLAGS="-w 0"

export CONTAINER_IMAGE=$CONTAINER_IMAGE
export RESOURCE_NAME=${RESOURCE_NAME:-"projects/735463103342/secrets/test-secret-a/versions/latest"}
export FILE_NAME=${FILE_NAME:-"secret"}
export SECRET_VALUE=${SECRET_VALUE:-"aHVudGVyMg=="}

setup() {
  if [[ -z "${GCP_SA_JSON}" ]]; then
    echo "Error: GCP Service Account (GCP_SA_JSON) is not provided" >&2
    return 1
  fi
}

@test "install gcp provider" {	
  run kubectl apply -f $PROVIDER_YAML --namespace $PROVIDER_NAMESPACE
  assert_success	

  cmd="kubectl wait --for=condition=Ready --timeout=60s pod -l app=csi-secrets-store-provider-gcp --namespace $PROVIDER_NAMESPACE"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  GCP_PROVIDER_POD=$(kubectl get pod --namespace $PROVIDER_NAMESPACE -l app=csi-secrets-store-provider-gcp -o jsonpath="{.items[0].metadata.name}")	

  run kubectl get pod/$GCP_PROVIDER_POD --namespace $PROVIDER_NAMESPACE
  assert_success
}

@test "create gcp k8s secret for provider auth" {
  run kubectl create secret generic secrets-store-creds --namespace $NAMESPACE --from-literal=key.json="${GCP_SA_JSON}"
  assert_success
}

@test "secretproviderclasses crd is established" {
  cmd="kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get crd/secretproviderclasses.secrets-store.csi.x-k8s.io
  assert_success
}

@test "deploy gcp secretproviderclass crd" {
  envsubst < $BATS_TESTS_DIR/gcp_v1alpha1_secretproviderclass.yaml | kubectl apply -f -

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/gcp -o yaml | grep gcp"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
  envsubst < $BATS_TESTS_DIR/nginx-pod-secrets-store-inline-volume-crd.yaml | kubectl apply -f -
  
  cmd="kubectl wait --for=condition=Ready --timeout=60s pod/nginx-secrets-store-inline-crd"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/nginx-secrets-store-inline-crd
  assert_success
}

@test "CSI inline volume test with pod portability - read gcp kv secret from pod" {
  result=$(kubectl exec nginx-secrets-store-inline-crd -- $EXEC_COMMAND /mnt/secrets-store/$FILE_NAME)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]
}

@test "Test filtered watch for nodePublishSecretRef" {
  run helm upgrade csi-secrets-store manifest_staging/charts/secrets-store-csi-driver --reuse-values --set filteredWatchSecret=true --wait --timeout=5m -v=5 --debug
  assert_success

  kubectl create ns filtered-watch
  kubectl create secret generic secrets-store-creds --from-literal=key.json="${GCP_SA_JSON}" -n filtered-watch
  # label the node publish secret ref secret
  run kubectl label secret secrets-store-creds secrets-store.csi.k8s.io/used=true -n filtered-watch
  assert_success

  envsubst < $BATS_TESTS_DIR/gcp_v1alpha1_secretproviderclass.yaml | kubectl apply -n filtered-watch -f -
  envsubst < $BATS_TESTS_DIR/nginx-pod-secrets-store-inline-volume-crd.yaml | kubectl apply -n filtered-watch -f -

  cmd="kubectl wait -n filtered-watch --for=condition=Ready --timeout=60s pod/nginx-secrets-store-inline-crd"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/nginx-secrets-store-inline-crd -n filtered-watch
  assert_success

  result=$(kubectl exec nginx-secrets-store-inline-crd -- $EXEC_COMMAND /mnt/secrets-store/$FILE_NAME)
  [[ "${result//$'\r'}" == "${SECRET_VALUE}" ]]
}
