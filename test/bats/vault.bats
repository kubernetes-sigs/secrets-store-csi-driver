#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests
WAIT_TIME=60
SLEEP_TIME=1
IMAGE_TAG=e2e-$(git rev-parse --short HEAD)


@test "install helm chart with e2e image" {
  run helm install charts/secrets-store-csi-driver -n csi-secrets-store --namespace dev \
          --set image.pullPolicy="IfNotPresent" \
          --set image.repository="e2e/secrets-store-csi" \
          --set image.tag=$IMAGE_TAG
  assert_success
}

@test "install vault" {
  run kubectl apply -f pkg/providers/vault/examples/vault.yaml
  assert_success
}

@test "setup vault" {
  VAULT_POD=$(kubectl get pod -l app=vault -o jsonpath="{.items[0].metadata.name}")
  kubectl port-forward ${VAULT_POD} 8200:8200 &

  export VAULT_ADDR="http://127.0.0.1:8200"
  export VAULT_TOKEN="root"
  kubectl create serviceaccount vault-auth

  kubectl apply -f - <<EOH
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
  EOH


  CLUSTER_NAME="$(kubectl config view --raw \
    -o go-template="{{ range .contexts }}{{ if eq .name \"$(kubectl config current-context)\" }}{{ index .context \"cluster\" }}{{ end }}{{ end }}")"

  SECRET_NAME="$(kubectl get serviceaccount vault-auth \
    -o go-template='{{ (index .secrets 0).name }}')"

  TR_ACCOUNT_TOKEN="$(kubectl get secret ${SECRET_NAME} \
    -o go-template='{{ .data.token }}' | base64 --decode)"

  K8S_HOST="https://$(kubectl get svc kubernetes -o go-template="{{ .spec.clusterIP }}")"

  K8S_CACERT="$(kubectl config view --raw \
    -o go-template="{{ range .clusters }}{{ if eq .name \"${CLUSTER_NAME}\" }}{{ index .cluster \"certificate-authority-data\" }}{{ end }}{{ end }}" | base64 --decode)"


  vault auth enable kubernetes

  vault write auth/kubernetes/config \
    kubernetes_host="${K8S_HOST}" \
    kubernetes_ca_cert="${K8S_CACERT}" \
    token_reviewer_jwt="${TR_ACCOUNT_TOKEN}"

  echo 'path "secret/data/foo" {
    capabilities = ["read", "list"]
  }

  path "sys/renew/*" {
    capabilities = ["update"]
  }' | vault policy write example-readonly -


  vault write auth/kubernetes/role/example-role \
    bound_service_account_names=csi-driver-registrar \
    bound_service_account_namespaces=default \
    policies=default,example-readonly \
    ttl=20m

  vault kv put secret/foo bar=hello
}





