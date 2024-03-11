#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests/e2e_provider_ssc
WAIT_TIME=60
SLEEP_TIME=1

@test "secretproviderclasses crd is established" {
  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  run kubectl get crd/secretproviderclasses.secrets-store.csi.x-k8s.io
  assert_success
}

@test "secretsync crd is established" {
  kubectl wait --for condition=established --timeout=60s crd/secretsyncs.secret-sync.x-k8s.io

  run kubectl get crd/secretsyncs.secret-sync.x-k8s.io
  assert_success
}

@test "Test rbac roles and role bindings exist" {
  run kubectl get clusterrole/secret-sync-controller-manager-role
  assert_success

  run kubectl get clusterrolebinding/secret-sync-controller-manager-rolebinding
  assert_success
}

@test "[v1alpha1] deploy e2e-providerspc secretproviderclass crd" {
  kubectl create namespace test-v1alpha1 --dry-run=client -o yaml | kubectl apply -f -

  envsubst <  $BATS_TESTS_DIR/e2e_provider_v1_secretproviderclass.yaml | kubectl apply -n test-v1alpha1 -f -

  kubectl wait --for condition=established -n test-v1alpha1 --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-providerspc -n test-v1alpha1 -o yaml | grep e2e-providerspc"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "[v1alpha1] deploy e2e-providerspc secretsync crd" {
  # Create the SPC
  envsubst < $BATS_TESTS_DIR/e2e_provider_v1_secretproviderclass.yaml | kubectl apply -n test-v1alpha1 -f -

  kubectl wait --for condition=established -n test-v1alpha1 --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/e2e-providerspc -n test-v1alpha1 -o yaml | grep e2e-providerspc"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  # Create the SecretSync
  envsubst < $BATS_TESTS_DIR/e2e_provider_v1alpha1_secretsync.yaml | kubectl apply -n test-v1alpha1 -f -

  kubectl wait --for condition=established -n test-v1alpha1 --timeout=60s crd/secretsyncs.secret-sync.x-k8s.io

  cmd="kubectl get secretsyncs.secret-sync.x-k8s.io/sse2esecret -n test-v1alpha1 -o yaml | grep sse2esecret"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  # Retrieve the secret 
  cmd="kubectl get secret sse2esecret -n test-v1alpha1 -o yaml | grep 'apiVersion: secret-sync.x-k8s.io/v1alpha1'"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

teardown_file() {
  if [[ "${INPLACE_UPGRADE_TEST}" != "true" ]]; then
    #cleanup
    run kubectl delete namespace test-ns
    run kubectl delete namespace test-v1alpha1
  fi
}
