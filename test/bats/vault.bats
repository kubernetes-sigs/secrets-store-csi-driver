#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests/vault
WAIT_TIME=120
SLEEP_TIME=1
NAMESPACE=default
PROVIDER_YAML=https://raw.githubusercontent.com/hashicorp/vault-csi-provider/e0eae762c669658b55cf458dedaddf53277af759/deployment/provider-vault-installer.yaml

export CONTAINER_IMAGE=nginx
export LABEL_VALUE=${LABEL_VALUE:-"test"}

@test "install vault provider" {
  run kubectl apply -f $PROVIDER_YAML --namespace $NAMESPACE
  assert_success

  cmd="kubectl wait --for=condition=Ready --timeout=60s pod -l app=csi-secrets-store-provider-vault --namespace $NAMESPACE"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  VAULT_PROVIDER_POD=$(kubectl get pod --namespace $NAMESPACE -l app=csi-secrets-store-provider-vault -o jsonpath="{.items[0].metadata.name}")

  run kubectl get pod/$VAULT_PROVIDER_POD --namespace $NAMESPACE
  assert_success
}

@test "install vault service account" {
  run kubectl create serviceaccount vault-auth
  assert_success

  cat <<EOF | kubectl apply -f -
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: role-tokenreview-binding
  namespace: default
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: ServiceAccount
  name: vault-auth
  namespace: default
EOF
}

@test "install vault" {
  run kubectl apply -f $BATS_TESTS_DIR/vault.yaml
  assert_success

  cmd="kubectl wait --for=condition=Ready --timeout=60s pod -l app=vault"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  VAULT_POD=$(kubectl get pod -l app=vault -o jsonpath="{.items[0].metadata.name}")

  run kubectl get pod/$VAULT_POD
  assert_success
}

@test "setup vault" {
  VAULT_POD=$(kubectl get pod -l app=vault -o jsonpath="{.items[0].metadata.name}")
  run kubectl exec $VAULT_POD -- vault auth enable kubernetes
  assert_success

  CLUSTER_NAME="$(kubectl config view --raw \
    -o go-template="{{ range .contexts }}{{ if eq .name \"$(kubectl config current-context)\" }}{{ index .context \"cluster\" }}{{ end }}{{ end }}")"

  SECRET_NAME="$(kubectl get serviceaccount vault-auth \
    -o go-template='{{ (index .secrets 0).name }}')"

  export TR_ACCOUNT_TOKEN="$(kubectl get secret ${SECRET_NAME} \
    -o go-template='{{ .data.token }}' | base64 --decode)"

  export K8S_HOST="https://$(kubectl get svc kubernetes -o go-template="{{ .spec.clusterIP }}")"

  export K8S_CACERT="$(kubectl config view --raw \
    -o go-template="{{ range .clusters }}{{ if eq .name \"${CLUSTER_NAME}\" }}{{ index .cluster \"certificate-authority-data\" }}{{ end }}{{ end }}" | base64 --decode)"

  run kubectl exec $VAULT_POD -- vault write auth/kubernetes/config \
    kubernetes_host="${K8S_HOST}" \
    kubernetes_ca_cert="${K8S_CACERT}" \
    token_reviewer_jwt="${TR_ACCOUNT_TOKEN}"
  assert_success

  run kubectl exec -ti $VAULT_POD -- vault policy write example-readonly -<<EOF
path "sys/mounts" {
  capabilities = ["read"]
}

path "secret/data/foo" {
  capabilities = ["read", "list"]
}

path "secret/data/foo1" {
  capabilities = ["read", "list"]
}

path "sys/renew/*" {
  capabilities = ["update"]
}
EOF
  assert_success

  run kubectl exec $VAULT_POD -- vault write auth/kubernetes/role/example-role \
    bound_service_account_names=secrets-store-csi-driver-provider-vault \
    bound_service_account_namespaces=$NAMESPACE \
    policies=default,example-readonly \
    ttl=20m
  assert_success

  run kubectl exec $VAULT_POD -- vault kv put secret/foo bar=hello
  assert_success

  run kubectl exec $VAULT_POD -- vault kv put secret/foo1 bar1=hello1
  assert_success
}

@test "secretproviderclasses crd is established" {
  cmd="kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get crd/secretproviderclasses.secrets-store.csi.x-k8s.io
  assert_success
}

@test "deploy vault secretproviderclass crd" {
  export VAULT_SERVICE_IP=$(kubectl get service vault -o jsonpath='{.spec.clusterIP}')

  envsubst < $BATS_TESTS_DIR/vault_v1alpha1_secretproviderclass.yaml | kubectl apply -f -

  cmd="kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/vault-foo -o yaml | grep vault"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
  envsubst < $BATS_TESTS_DIR/nginx-pod-vault-inline-volume-secretproviderclass.yaml | kubectl apply -f -

  cmd="kubectl wait --for=condition=Ready --timeout=60s pod/nginx-secrets-store-inline"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/nginx-secrets-store-inline
  assert_success
}

@test "CSI inline volume test with pod portability - read vault secret from pod" {
  result=$(kubectl exec nginx-secrets-store-inline -- cat /mnt/secrets-store/bar)
  [[ "$result" == "hello" ]]

  result=$(kubectl exec nginx-secrets-store-inline -- cat /mnt/secrets-store/bar1)
  [[ "$result" == "hello1" ]]
}

@test "Sync with K8s secrets - create deployment" {
  export VAULT_SERVICE_IP=$(kubectl get service vault -o jsonpath='{.spec.clusterIP}')

  envsubst < $BATS_TESTS_DIR/vault_synck8s_v1alpha1_secretproviderclass.yaml | kubectl apply -f -

  cmd="kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/vault-foo-sync -o yaml | grep vault"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl apply -f $BATS_TESTS_DIR/nginx-deployment-synck8s.yaml
  assert_success

  run kubectl apply -f $BATS_TESTS_DIR/nginx-deployment-two-synck8s.yaml
  assert_success

  cmd="kubectl wait --for=condition=Ready --timeout=60s pod -l app=nginx"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences with multiple owners" {
  POD=$(kubectl get pod -l app=nginx -o jsonpath="{.items[0].metadata.name}")
  result=$(kubectl exec $POD -- cat /mnt/secrets-store/bar)
  [[ "$result" == "hello" ]]

  result=$(kubectl exec $POD -- cat /mnt/secrets-store/bar1)
  [[ "$result" == "hello1" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.data.pwd}" | base64 -d)
  [[ "$result" == "hello" ]]

  result=$(kubectl exec $POD -- printenv | grep SECRET_USERNAME | awk -F"=" '{ print $2 }' | tr -d '\r\n')
  [[ "$result" == "hello1" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.metadata.labels.environment}")
  [[ "${result//$'\r'}" == "${LABEL_VALUE}" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.metadata.labels.secrets-store\.csi\.k8s\.io/managed}")
  [[ "${result//$'\r'}" == "true" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret default 4"
  assert_success
}

@test "Sync with K8s secrets - delete deployment, check secret is deleted" {
  run kubectl delete -f $BATS_TESTS_DIR/nginx-deployment-synck8s.yaml
  assert_success
  
  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret default 2"
  assert_success

  run kubectl delete -f $BATS_TESTS_DIR/nginx-deployment-two-synck8s.yaml
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted foosecret default"
  assert_success

  run kubectl delete -f $BATS_TESTS_DIR/vault_synck8s_v1alpha1_secretproviderclass.yaml
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - create deployment" {
  export VAULT_SERVICE_IP=$(kubectl get service vault -o jsonpath='{.spec.clusterIP}')

  run kubectl create ns test-ns
  assert_success

  envsubst < $BATS_TESTS_DIR/vault_v1alpha1_secretproviderclass_ns.yaml | kubectl apply -f -

  cmd="kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/vault-foo-sync -o yaml | grep vault"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/vault-foo-sync -n test-ns -o yaml | grep vault"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  envsubst < $BATS_TESTS_DIR/nginx-deployment-synck8s.yaml | kubectl apply -n test-ns -f -

  cmd="kubectl wait --for=condition=Ready --timeout=60s pod -l app=nginx -n test-ns"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "Test Namespaced scope SecretProviderClass - Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences" {
  POD=$(kubectl get pod -l app=nginx -n test-ns -o jsonpath="{.items[0].metadata.name}")
  result=$(kubectl exec -n test-ns $POD -- cat /mnt/secrets-store/bar)
  [[ "$result" == "hello" ]]

  result=$(kubectl exec -n test-ns $POD -- cat /mnt/secrets-store/bar1)
  [[ "$result" == "hello1" ]]

  result=$(kubectl get secret foosecret -n test-ns -o jsonpath="{.data.pwd}" | base64 -d)
  [[ "$result" == "hello" ]]

  result=$(kubectl exec -n test-ns $POD -- printenv | grep SECRET_USERNAME | awk -F"=" '{ print $2 }' | tr -d '\r\n')
  [[ "$result" == "hello1" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret test-ns 2"
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - Sync with K8s secrets - delete deployment, check secret deleted" {
  run kubectl delete -f $BATS_TESTS_DIR/nginx-deployment-synck8s.yaml -n test-ns
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted foosecret test-ns"
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - Should fail when no secret provider class in same namespace" {
  export VAULT_SERVICE_IP=$(kubectl get service vault -o jsonpath='{.spec.clusterIP}')

  run kubectl create ns negative-test-ns
  assert_success

  envsubst < $BATS_TESTS_DIR/nginx-deployment-synck8s.yaml | kubectl apply -n negative-test-ns -f -
  sleep 5

  POD=$(kubectl get pod -l app=nginx -n negative-test-ns -o jsonpath="{.items[0].metadata.name}")
  cmd="kubectl describe pod $POD -n negative-test-ns | grep 'FailedMount.*failed to get secretproviderclass negative-test-ns/vault-foo-sync.*not found'"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl delete -f $BATS_TESTS_DIR/nginx-deployment-synck8s.yaml -n negative-test-ns
  assert_success

  run kubectl delete ns negative-test-ns
  assert_success
}

@test "deploy multiple vault secretproviderclass crd" {
  export VAULT_SERVICE_IP=$(kubectl get service vault -o jsonpath='{.spec.clusterIP}')

  envsubst < $BATS_TESTS_DIR/vault_v1alpha1_multiple_secretproviderclass.yaml | kubectl apply -f -

  cmd="kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/vault-foo-sync-0 -o yaml | grep vault-foo-sync-0"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/vault-foo-sync-1 -o yaml | grep vault-foo-sync-1"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "deploy pod with multiple secret provider class" {
  envsubst < $BATS_TESTS_DIR/nginx-pod-vault-inline-volume-multiple-spc.yaml | kubectl apply -f -
  
  cmd="kubectl wait --for=condition=Ready --timeout=60s pod/nginx-secrets-store-inline-multiple-crd"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/nginx-secrets-store-inline-multiple-crd
  assert_success
}

@test "CSI inline volume test with multiple secret provider class" {
  result=$(kubectl exec nginx-secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-0/bar)
  [[ "$result" == "hello" ]]

  result=$(kubectl exec nginx-secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-0/bar1)
  [[ "$result" == "hello1" ]]

  result=$(kubectl get secret foosecret-0 -o jsonpath="{.data.pwd}" | base64 -d)
  [[ "$result" == "hello" ]]

  result=$(kubectl exec nginx-secrets-store-inline-multiple-crd -- printenv | grep SECRET_USERNAME_0 | awk -F"=" '{ print $2 }' | tr -d '\r\n')
  [[ "$result" == "hello1" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret-0 default 1"
  assert_success

  result=$(kubectl exec nginx-secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-1/bar)
  [[ "$result" == "hello" ]]

  result=$(kubectl exec nginx-secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-1/bar1)
  [[ "$result" == "hello1" ]]

  result=$(kubectl get secret foosecret-1 -o jsonpath="{.data.pwd}" | base64 -d)
  [[ "$result" == "hello" ]]

  result=$(kubectl exec nginx-secrets-store-inline-multiple-crd -- printenv | grep SECRET_USERNAME_1 | awk -F"=" '{ print $2 }' | tr -d '\r\n')
  [[ "$result" == "hello1" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret-1 default 1"
  assert_success
}
