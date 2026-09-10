#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=tests/fortanix
WAIT_TIME=120
SLEEP_TIME=1
PROVIDER_YAML=https://raw.githubusercontent.com/fortanix/fortanix-csi-provider/main/deployment/fortanix-csi-provider.yaml
NAMESPACE=kube-system
export FORTANIX_SECRET_VALUE=${FORTANIX_SECRET_VALUE:-"test-value"}

setup() {
  if [[ -z "${FORTANIX_API_KEY}" ]]; then
    echo "Error: Fortanix API Key is not provided" >&2
    return 1
  fi
  if [[ -z "${FORTANIX_DSM_ENDPOINT}" ]]; then
    echo "Error: Fortanix DSM Endpoint is not provided" >&2
    return 1
  fi
  if [[ -z "${FORTANIX_SECRET_NAME}" ]]; then
    echo "Error: Fortanix Secret Name is not provided" >&2
    return 1
  fi
}

@test "install fortanix csi provider" {
  run kubectl apply -f $PROVIDER_YAML
  assert_success

  kubectl --namespace $NAMESPACE wait --for=condition=Ready --timeout=120s pod -l app=fortanix-csi-provider

  PROVIDER_POD=$(kubectl --namespace $NAMESPACE get pod -l app=fortanix-csi-provider -o jsonpath="{.items[0].metadata.name}")
  run kubectl --namespace $NAMESPACE get pod/$PROVIDER_POD
  assert_success
}

@test "create fortanix api key secret" {
  # Delete existing secret if it exists
  kubectl delete secret fortanix-api-key -n $NAMESPACE --ignore-not-found=true
  run kubectl create secret generic fortanix-api-key --from-literal=api-key=${FORTANIX_API_KEY} -n $NAMESPACE
  assert_success
}

@test "secretproviderclasses crd is established" {
  kubectl wait --for condition=established --timeout=${WAIT_TIME}s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  run kubectl get crd/secretproviderclasses.secrets-store.csi.x-k8s.io
  assert_success
}

@test "deploy fortanix secretproviderclass crd" {
  envsubst < $BATS_TESTS_DIR/secretproviderclass.yaml | kubectl apply -f -

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/fortanix-test -o yaml | grep fortanix-csi-provider"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
  envsubst < $BATS_TESTS_DIR/pod-inline-volume-secretproviderclass.yaml | kubectl apply -f -

  # wait for pod to be running
  kubectl wait --for=condition=Ready --timeout=${WAIT_TIME}s pod/secrets-store-inline

  run kubectl get pod/secrets-store-inline
  assert_success
}

@test "CSI inline volume test with pod portability - read fortanix secret from pod" {
  result=$(kubectl exec secrets-store-inline -- cat /mnt/secrets-store/$FORTANIX_SECRET_NAME)
  [[ "${result}" == "$FORTANIX_SECRET_VALUE" ]]
}

@test "CSI inline volume test with pod portability - unmount" {
  # On Linux a failure to unmount the tmpfs will block the pod from being deleted.
  run kubectl delete pod secrets-store-inline
  assert_success

  run kubectl wait --for=delete --timeout=${WAIT_TIME}s pod/secrets-store-inline

  # Sleep to allow time for logs to propagate.
  sleep 10

  # save debug information to archive in case of failure
  archive_info

  run bash -c "kubectl logs -l app=secrets-store-csi-driver --tail -1 -c secrets-store -n kube-system | grep '^E.*failed to clean and unmount target path.*$'"
  assert_failure
}

teardown_file() {
  archive_provider "app=fortanix-csi-provider" || true
  archive_info || true
}
