#!/usr/bin/env bats

load helpers

WAIT_TIME=120
SLEEP_TIME=1
PROVIDER_YAML=https://raw.githubusercontent.com/aws/secrets-store-csi-driver-provider-aws/main/deployment/aws-provider-installer.yaml
NAMESPACE=kube-system
POD_NAME=basic-test-mount
export REGION=${REGION:-us-west-2}

export ACCOUNT_NUMBER=$(aws --region $REGION  sts get-caller-identity --query Account --output text)
BATS_TEST_DIR=test/bats/tests/aws


if [ -z "$UUID" ]; then 
   export UUID=secret-$(openssl rand -hex 6) 
fi 

export SM_TEST_1_NAME=SecretsManagerTest1-$UUID 
export SM_TEST_2_NAME=SecretsManagerTest2-$UUID 
export SM_SYNC_NAME=SecretsManagerSync-$UUID
export SM_ROT_TEST_NAME=SecretsManagerRotationTest-$UUID

export PM_TEST_1_NAME=ParameterStoreTest1-$UUID
export PM_TEST_LONG_NAME=ParameterStoreTestWithLongName-$UUID 
export PM_ROTATION_TEST_NAME=ParameterStoreRotationTest-$UUID

setup_file() {
   #Create test secrets
   aws secretsmanager create-secret --name $SM_TEST_1_NAME --secret-string SecretsManagerTest1Value --region $REGION
   aws secretsmanager create-secret --name $SM_TEST_2_NAME --secret-string SecretsManagerTest2Value --region $REGION
   aws secretsmanager create-secret --name $SM_SYNC_NAME --secret-string SecretUser --region $REGION

   aws ssm put-parameter --name $PM_TEST_1_NAME --value ParameterStoreTest1Value --type SecureString --region $REGION
   aws ssm put-parameter --name $PM_TEST_LONG_NAME --value ParameterStoreTest2Value --type SecureString --region $REGION

   aws ssm put-parameter --name $PM_ROTATION_TEST_NAME --value BeforeRotation --type SecureString --region $REGION
   aws secretsmanager create-secret --name $SM_ROT_TEST_NAME --secret-string BeforeRotation --region $REGION
}

teardown_file() {
    aws secretsmanager delete-secret --secret-id $SM_TEST_1_NAME --force-delete-without-recovery --region $REGION
    aws secretsmanager delete-secret --secret-id $SM_TEST_2_NAME --force-delete-without-recovery --region $REGION
    aws secretsmanager delete-secret --secret-id $SM_SYNC_NAME --force-delete-without-recovery --region $REGION

    aws ssm delete-parameter --name $PM_TEST_1_NAME --region $REGION
    aws ssm delete-parameter --name $PM_TEST_LONG_NAME --region $REGION 

    aws ssm delete-parameter --name $PM_ROTATION_TEST_NAME --region $REGION
    aws secretsmanager delete-secret --secret-id $SM_ROT_TEST_NAME --force-delete-without-recovery --region $REGION
}

@test "Install aws provider" {
    run kubectl --namespace $NAMESPACE apply -f $PROVIDER_YAML  
    assert_success

    cmd="kubectl --namespace $NAMESPACE wait --for=condition=Ready --timeout=60s pod -l app=csi-secrets-store-provider-aws"
    wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

    PROVIDER_POD=$(kubectl --namespace $NAMESPACE get pod -l app=csi-secrets-store-provider-aws -o jsonpath="{.items[0].metadata.name}")	
    run kubectl --namespace $NAMESPACE get pod/$PROVIDER_POD
    assert_success
}

@test "secretproviderclasses crd is established" {
    cmd="kubectl wait --namespace $NAMESPACE --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io"
    wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

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

@test "deploy aws secretproviderclass crd" {
    envsubst < $BATS_TEST_DIR/BasicTestMountSPC.yaml | kubectl --namespace $NAMESPACE apply -f -

    cmd="kubectl --namespace $NAMESPACE get secretproviderclasses.secrets-store.csi.x-k8s.io/basic-test-mount-spc -o yaml | grep aws"
    wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
   kubectl --namespace $NAMESPACE  apply -f $BATS_TEST_DIR/BasicTestMount.yaml
   cmd="kubectl --namespace $NAMESPACE  wait --for=condition=Ready --timeout=60s pod/basic-test-mount"
   wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

   run kubectl --namespace $NAMESPACE  get pod/$POD_NAME
   assert_success
}

@test "CSI inline volume test with rotation - parameter store" {
   result=$(kubectl --namespace $NAMESPACE exec $POD_NAME -- cat /mnt/secrets-store/$PM_ROTATION_TEST_NAME)
   [[ "${result//$'\r'}" == "BeforeRotation" ]]

   aws ssm put-parameter --name $PM_ROTATION_TEST_NAME --value AfterRotation --type SecureString --overwrite --region $REGION
   sleep 40
   result=$(kubectl --namespace $NAMESPACE exec $POD_NAME -- cat /mnt/secrets-store/$PM_ROTATION_TEST_NAME)
   [[ "${result//$'\r'}" == "AfterRotation" ]]
}

@test "CSI inline volume test with rotation - secrets manager" {
   result=$(kubectl --namespace $NAMESPACE exec $POD_NAME -- cat /mnt/secrets-store/$SM_ROT_TEST_NAME)
   [[ "${result//$'\r'}" == "BeforeRotation" ]]
  
   aws secretsmanager put-secret-value --secret-id $SM_ROT_TEST_NAME --secret-string AfterRotation --region $REGION
   sleep 40
   result=$(kubectl --namespace $NAMESPACE exec $POD_NAME -- cat /mnt/secrets-store/$SM_ROT_TEST_NAME)
   [[ "${result//$'\r'}" == "AfterRotation" ]]
}

@test "CSI inline volume test with pod portability - read ssm parameters from pod" {
   result=$(kubectl --namespace $NAMESPACE exec $POD_NAME -- cat /mnt/secrets-store/$PM_TEST_1_NAME)
   [[ "${result//$'\r'}" == "ParameterStoreTest1Value" ]]

   result=$(kubectl --namespace $NAMESPACE exec $POD_NAME -- cat /mnt/secrets-store/ParameterStoreTest2)
   [[ "${result//$'\r'}" == "ParameterStoreTest2Value" ]]
}

@test "CSI inline volume test with pod portability - read secrets manager secrets from pod" {
    result=$(kubectl --namespace $NAMESPACE exec $POD_NAME -- cat /mnt/secrets-store/$SM_TEST_1_NAME)
    [[ "${result//$'\r'}" == "SecretsManagerTest1Value" ]]
   
    result=$(kubectl --namespace $NAMESPACE exec $POD_NAME -- cat /mnt/secrets-store/SecretsManagerTest2)
    [[ "${result//$'\r'}" == "SecretsManagerTest2Value" ]]        
}

@test "Sync with Kubernetes Secret" { 
    run kubectl get secret --namespace $NAMESPACE secret
    assert_success

    result=$(kubectl --namespace=$NAMESPACE get secret secret -o jsonpath="{.data.username}" | base64 -d)
    [[ "$result" == "SecretUser" ]]
}

@test "Sync with Kubernetes Secret - Delete deployment. Secret should also be deleted" {  
    run kubectl --namespace $NAMESPACE delete -f $BATS_TEST_DIR/BasicTestMount.yaml
    assert_success

    run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted secret $NAMESPACE"
    assert_success
}
