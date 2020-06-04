#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests
WAIT_TIME=60
SLEEP_TIME=1
IMAGE_TAG=v0.0.8-e2e-$(git rev-parse --short HEAD)
NAMESPACE=default
PROVIDER_YAML=https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/deployment/provider-azure-installer.yaml
CONTAINER_IMAGE=nginx
EXEC_COMMAND="cat /mnt/secrets-store"
BASE64_FLAGS="-w 0"
if [[ "$OSTYPE" == *"darwin"* ]]; then
  BASE64_FLAGS="-b 0"
fi

if [ $TEST_WINDOWS ]; then
  PROVIDER_YAML=https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/deployment/provider-azure-installer-windows.yaml
  CONTAINER_IMAGE=mcr.microsoft.com/windows/servercore/iis:windowsservercore-ltsc2019
  EXEC_COMMAND="powershell.exe cat /mnt/secrets-store"
fi

export KEYVAULT_NAME=${KEYVAULT_NAME:-csi-secrets-store-e2e}
export SECRET_NAME=${KEYVAULT_SECRET_NAME:-secret1}
export SECRET_VERSION=${KEYVAULT_SECRET_VERSION:-""}
export SECRET_VALUE=${KEYVAULT_SECRET_VALUE:-"test"}
export KEY_NAME=${KEYVAULT_KEY_NAME:-key1}
export KEY_VERSION=${KEYVAULT_KEY_VERSION:-7cc095105411491b84fe1b92ebbcf01a}
export KEY_VALUE_CONTAINS=${KEYVAULT_KEY_VALUE:-"LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUlJQklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUF4K2FadlhJN2FldG5DbzI3akVScgpheklaQ2QxUlBCQVZuQU1XcDhqY05TQk5MOXVuOVJrenJHOFd1SFBXUXNqQTA2RXRIOFNSNWtTNlQvaGQwMFNRCk1aODBMTlNxYkkwTzBMcWMzMHNLUjhTQ0R1cEt5dkpkb01LSVlNWHQzUlk5R2Ywam1ucHNKOE9WbDFvZlRjOTIKd1RINXYyT2I1QjZaMFd3d25MWlNiRkFnSE1uTHJtdEtwZTVNcnRGU21nZS9SL0J5ZXNscGU0M1FubnpndzhRTwpzU3ZMNnhDU21XVW9WQURLL1MxREU0NzZBREM2a2hGTjF5ZHUzbjVBcnREVGI0c0FjUHdTeXB3WGdNM3Y5WHpnClFKSkRGT0JJOXhSTW9UM2FjUWl0Z0c2RGZibUgzOWQ3VU83M0o3dUFQWUpURG1pZGhrK0ZFOG9lbjZWUG9YRy8KNXdJREFRQUIKLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0t"}
export CONTAINER_IMAGE=$CONTAINER_IMAGE

setup() {
  if [[ -z "${AZURE_CLIENT_ID}" ]] || [[ -z "${AZURE_CLIENT_SECRET}" ]]; then
    echo "Error: Azure service principal is not provided" >&2
    return 1
  fi
}

@test "install azure provider" {	
  run kubectl apply -f $PROVIDER_YAML --namespace $NAMESPACE	
  assert_success	

  AZURE_PROVIDER_POD=$(kubectl get pod --namespace $NAMESPACE -l app=csi-secrets-store-provider-azure -o jsonpath="{.items[0].metadata.name}")	
  cmd="kubectl wait --for=condition=Ready --timeout=60s pod/$AZURE_PROVIDER_POD --namespace $NAMESPACE"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/$AZURE_PROVIDER_POD --namespace $NAMESPACE
  assert_success
}

@test "create azure k8s secret" {
  run kubectl create secret generic secrets-store-creds --from-literal clientid=${AZURE_CLIENT_ID} --from-literal clientsecret=${AZURE_CLIENT_SECRET}
  assert_success
}

@test "CSI inline volume test" {
  skip
  envsubst < $BATS_TESTS_DIR/nginx-pod-secrets-store-inline-volume.yaml | kubectl apply -f -

  cmd="kubectl wait --for=condition=Ready --timeout=60s pod/nginx-secrets-store-inline"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/nginx-secrets-store-inline
  assert_success
}

@test "CSI inline volume test - read azure kv secret from pod" {
  skip
  result=$(kubectl exec nginx-secrets-store-inline -- $EXEC_COMMAND/$SECRET_NAME)
  [[ "${result//$'\r'}" -eq "${SECRET_VALUE}" ]]
}

@test "CSI inline volume test - read azure kv key from pod" {
  skip
  result=$(kubectl exec nginx-secrets-store-inline -- $EXEC_COMMAND/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]
}

@test "secretproviderclasses crd is established" {
  cmd="kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get crd/secretproviderclasses.secrets-store.csi.x-k8s.io
  assert_success
}

@test "deploy azure secretproviderclass crd" {
  envsubst < $BATS_TESTS_DIR/azure_v1alpha1_secretproviderclass.yaml | kubectl apply -f -

  cmd="kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure -o yaml | grep azure"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
  envsubst < $BATS_TESTS_DIR/nginx-pod-secrets-store-inline-volume-crd.yaml | kubectl apply -f -
  
  cmd="kubectl wait --for=condition=Ready --timeout=60s pod/nginx-secrets-store-inline-crd"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/nginx-secrets-store-inline-crd
  assert_success
}

@test "CSI inline volume test with pod portability - read azure kv secret from pod" {
  result=$(kubectl exec nginx-secrets-store-inline-crd -- $EXEC_COMMAND/$SECRET_NAME)
  [[ "${result//$'\r'}" -eq "${SECRET_VALUE}" ]]
}

@test "CSI inline volume test with pod portability - read azure kv key from pod" {
  result=$(kubectl exec nginx-secrets-store-inline-crd -- $EXEC_COMMAND/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]
}

@test "Sync with K8s secrets - create deployment" {
  envsubst < $BATS_TESTS_DIR/azure_synck8s_v1alpha1_secretproviderclass.yaml | kubectl apply -f - 

  cmd="kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/azure-sync -o yaml | grep azure"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  envsubst < $BATS_TESTS_DIR/nginx-deployment-synck8s-azure.yaml | kubectl apply -f -

  cmd="kubectl wait --for=condition=Ready --timeout=60s pod -l app=nginx"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences" {
  POD=$(kubectl get pod -l app=nginx -o jsonpath="{.items[0].metadata.name}")

  result=$(kubectl exec $POD -- $EXEC_COMMAND/secretalias)
  [[ "${result//$'\r'}" -eq "${SECRET_VALUE}" ]]

  result=$(kubectl exec $POD -- $EXEC_COMMAND/$KEY_NAME)
  result_base64_encoded=$(echo "${result//$'\r'}" | base64 ${BASE64_FLAGS})
  [[ "${result_base64_encoded}" == *"${KEY_VALUE_CONTAINS}"* ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.data.username}" | base64 -d)
  [[ "${result//$'\r'}" -eq "${SECRET_VALUE}" ]]

  result=$(kubectl exec -it $POD printenv | grep SECRET_USERNAME) | awk -F"=" '{ print $2}'
  [[ "${result//$'\r'}" -eq "${SECRET_VALUE}" ]]

  result=$(kubectl get secret foosecret -o json | jq '.metadata.ownerReferences | length')
  [[ "$result" -eq "2" ]]
}

@test "Sync with K8s secrets - delete deployment, check secret deleted" {
  run kubectl delete -f $BATS_TESTS_DIR/nginx-deployment-synck8s-azure.yaml
  assert_success
  sleep 20
  result=$(kubectl get secret | grep foosecret | wc -l)
  [[ "$result" -eq "0" ]]
}
