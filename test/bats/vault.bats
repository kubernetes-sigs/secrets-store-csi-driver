#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests
WAIT_TIME=120
SLEEP_TIME=1
IMAGE_TAG=v0.0.8-e2e-$(git rev-parse --short HEAD)
NAMESPACE=default
PROVIDER_YAML=https://raw.githubusercontent.com/hashicorp/secrets-store-csi-driver-provider-vault/master/deployment/provider-vault-installer.yaml

export CONTAINER_IMAGE=nginx

@test "install vault provider" {
  run kubectl apply -f $PROVIDER_YAML --namespace $NAMESPACE
  assert_success

  VAULT_PROVIDER_POD=$(kubectl get pod --namespace $NAMESPACE -l app=csi-secrets-store-provider-vault -o jsonpath="{.items[0].metadata.name}")
  cmd="kubectl wait --for=condition=Ready --timeout=60s pod/$VAULT_PROVIDER_POD --namespace $NAMESPACE"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

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

  VAULT_POD=$(kubectl get pod -l app=vault -o jsonpath="{.items[0].metadata.name}")
  cmd="kubectl wait --for=condition=Ready --timeout=60s pod/$VAULT_POD"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl get pod/$VAULT_POD
  assert_success
}

@test "setup vault" {
  VAULT_POD=$(kubectl get pod -l app=vault -o jsonpath="{.items[0].metadata.name}")
  run kubectl exec -it $VAULT_POD -- vault auth enable kubernetes
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

  run kubectl exec -it $VAULT_POD -- vault write auth/kubernetes/config \
    kubernetes_host="${K8S_HOST}" \
    kubernetes_ca_cert="${K8S_CACERT}" \
    token_reviewer_jwt="${TR_ACCOUNT_TOKEN}"
  assert_success

  run kubectl exec -it $VAULT_POD -- vault policy write example-readonly -<<EOF
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

  run kubectl exec -it $VAULT_POD -- vault write auth/kubernetes/role/example-role \
    bound_service_account_names=secrets-store-csi-driver \
    bound_service_account_namespaces=$NAMESPACE \
    policies=default,example-readonly \
    ttl=20m
  assert_success

  run kubectl exec -it $VAULT_POD -- vault kv put secret/foo bar=hello
  assert_success

  run kubectl exec -it $VAULT_POD -- vault kv put secret/foo1 bar=hello1
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
  result=$(kubectl exec -it nginx-secrets-store-inline -- cat /mnt/secrets-store/foo)
  [[ "$result" == "hello" ]]

  result=$(kubectl exec -it nginx-secrets-store-inline -- cat /mnt/secrets-store/foo1)
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

  cmd="kubectl wait --for=condition=Ready --timeout=60s pod -l app=nginx"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences" {
  POD=$(kubectl get pod -l app=nginx -o jsonpath="{.items[0].metadata.name}")
  result=$(kubectl exec -it $POD -- cat /mnt/secrets-store/foo)
  [[ "$result" == "hello" ]]

  result=$(kubectl exec -it $POD -- cat /mnt/secrets-store/foo1)
  [[ "$result" == "hello1" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.data.pwd}" | base64 -d)
  [[ "$result" == "hello" ]]

  result=$(kubectl exec -it $POD -- printenv | grep SECRET_USERNAME | awk -F"=" '{ print $2 }' | tr -d '\r\n')
  [[ "$result" == "hello1" ]]

  result=$(kubectl get secret foosecret -o json | jq '.metadata.ownerReferences | length')
  [[ "$result" == "2" ]]
}

@test "Sync with K8s secrets - delete deployment, check secret is deleted" {
  run kubectl delete -f $BATS_TESTS_DIR/nginx-deployment-synck8s.yaml
  assert_success
  sleep 20
  result=$(kubectl get secret | grep foosecret | wc -l)
  [[ "$result" == "0" ]]
}
