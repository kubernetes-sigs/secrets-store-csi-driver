#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests/akeyless
WAIT_TIME=120
SLEEP_TIME=1


export LABEL_VALUE=${LABEL_VALUE:-"test"}
export ANNOTATION_VALUE=${ANNOTATION_VALUE:-"app=test"}

@test "install akeyless csi provider" {
  # create the ns akeyless
  kubectl create ns akeyless
  # install the akeyless provider using the helm charts

  helm repo add akeyless https://akeylesslabs.github.io/helm-charts
  helm repo update
  helm install akeyless akeyless/akeyless-csi-provider --namespace=akeyless

  # wait for akeyless and akeyless-csi-provider pods to be running
  kubectl wait --for=condition=Ready --timeout=120s pods --all -n akeyless
}

@test "secretproviderclasses crd is established" {
  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  run kubectl get crd/secretproviderclasses.secrets-store.csi.x-k8s.io
  assert_success
}

@test "deploy akeyless secretproviderclass crd" {
  kubectl apply -f $BATS_TESTS_DIR/secretproviderclass.yaml
  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/akeyless-test -o yaml | grep akeyless"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
  kubectl apply -f $BATS_TESTS_DIR/pod-inline-volume-secretproviderclass.yaml
  # wait for pod to be running
  kubectl wait --for=condition=Ready --timeout=60s pod/secrets-store-inline

  run kubectl get pod/secrets-store-inline
  assert_success
}

@test "CSI inline volume test with pod portability - read akeyless secret from pod" {
  result=$(kubectl exec secrets-store-inline -- cat /mnt/secrets-store/bar)
  [[ "$result" == "foo" ]]

  result=$(kubectl exec secrets-store-inline -- cat /mnt/secrets-store/bar1)
  [[ "$result" == "very-secret-value" ]]
}

@test "CSI inline volume test with pod portability - unmount succeeds" {
  # On Linux a failure to unmount the tmpfs will block the pod from being
  # deleted.
  run kubectl delete pod secrets-store-inline
  assert_success

  run kubectl wait --for=delete --timeout=${WAIT_TIME}s pod/secrets-store-inline
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

teardown_file() {
  archive_info || true
}
