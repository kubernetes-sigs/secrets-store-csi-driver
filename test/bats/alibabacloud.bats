#!/usr/bin/env bats

load helpers

WAIT_TIME=120
SLEEP_TIME=1
NAMESPACE=kube-system
POD_NAME=alibabacloud-basic-test-mount
BATS_TEST_DIR=test/bats/tests/alibabacloud

setup() {
  if [[ -z "${ALIBABACLOUD_ACCESS_KEY}" ]] || [[ -z "${ALIBABACLOUD_ACCESS_SECRET}" ]]; then
    echo "Error: ram ak/sk is not provided" >&2
    return 1
  fi
}

setup_file() {
    #Configure aliyun cli profile
    aliyun configure set --profile akProfile --mode AK --region us-west-1 --access-key-id ${ALIBABACLOUD_ACCESS_KEY} --access-key-secret ${ALIBABACLOUD_ACCESS_SECRET}

    #Create test secrets
    aliyun kms CreateSecret --SecretName testBasic --SecretData testValue --VersionId v1
}

teardown_file() {
    aliyun kms DeleteSecret --SecretName testBasic --ForceDeleteWithoutRecovery true
}

@test "secretproviderclasses crd is established" {
    cmd="kubectl wait --namespace $NAMESPACE --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io"
    wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

    run kubectl get crd/secretproviderclasses.secrets-store.csi.x-k8s.io
    assert_success
}

@test "create alibabacloud k8s secret" {
    run kubectl create secret generic secrets-store-creds --from-literal access_key=${ALIBABACLOUD_ACCESS_KEY} --from-literal access_secret=${ALIBABACLOUD_ACCESS_SECRET} --namespace=$NAMESPACE
    assert_success

    # label the node publish secret ref secret
    run kubectl label secret secrets-store-creds secrets-store.csi.k8s.io/used=true --namespace=$NAMESPACE
    assert_success
}

@test "deploy alibabacloud secretproviderclass crd" {
    envsubst < $BATS_TEST_DIR/secretproviderclass.yaml | kubectl --namespace $NAMESPACE apply -f -

    cmd="kubectl --namespace $NAMESPACE get secretproviderclasses.secrets-store.csi.x-k8s.io/alibabacloud-basic-test-mount-spc -o yaml | grep alibabacloud"
    wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
   kubectl --namespace $NAMESPACE  apply -f $BATS_TEST_DIR/pod-inline-volume-secretproviderclass.yaml
   cmd="kubectl --namespace $NAMESPACE  wait --for=condition=Ready --timeout=60s pod/alibabacloud-basic-test-mount"
   wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

   run kubectl --namespace $NAMESPACE  get pod/$POD_NAME
   assert_success
}

@test "CSI inline volume test with pod portability - read secrets manager secrets from pod" {
    result=$(kubectl --namespace $NAMESPACE exec $POD_NAME -- cat /mnt/secrets-store/testBasic)
    [[ "${result//$'\r'}" == "testValue" ]]
}
