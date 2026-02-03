#!/usr/bin/env bats

# mostly inspired by the vault provider tests
# https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/main/test/bats/vault.bats
# credits to @sozercan @aramase @ritazh and the rest of the community

load helpers

BATS_TESTS_DIR=test/bats/tests/openbao
WAIT_TIME=120
SLEEP_TIME=1

export LABEL_VALUE=${LABEL_VALUE:-"test"}
export ANNOTATION_VALUE=${ANNOTATION_VALUE:-"app=test"}

@test "install openbao provider" {
  # install openbao including the csi provider using helm
  helm repo add openbao https://openbao.github.io/openbao-helm
  helm repo update
  helm install openbao openbao/openbao -n openbao --create-namespace \
    --set "server.dev.enabled=true" \
    --set "injector.enabled=false" \
    --set "csi.enabled=true"

  # wait for openbao and openbao-csi-provider pods to be running
  kubectl wait --for=condition=Ready --timeout=120s pods --all -n openbao
}

@test "configure openbao" {
  # create the secrets pair in openbao
  kubectl exec openbao-0 -n openbao -- bao secrets enable -version=2 -path=secrets kv
  kubectl exec openbao-0 -n openbao -- bao kv put secrets/foo foo=openbao-foo
  kubectl exec openbao-0 -n openbao -- bao kv put secrets/bar bar=openbao-bar

  # enable authentication
  kubectl exec openbao-0 -n openbao -- bao auth enable kubernetes

  local token_reviewer_jwt="$(kubectl exec openbao-0 -n openbao -- cat /var/run/secrets/kubernetes.io/serviceaccount/token)"
  local kubernetes_service_ip="$(kubectl get svc kubernetes -o go-template="{{ .spec.clusterIP }}")"
  # enable authentication using the kubernetes service token from openbao pod
  kubectl exec -i openbao-0 -n openbao -- bao write auth/kubernetes/config \
    issuer="https://kubernetes.default.svc.cluster.local" \
    token_reviewer_jwt="${token_reviewer_jwt}" \
    kubernetes_host="https://${kubernetes_service_ip}:443" \
    kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt

  # create openbao policy to allow access to created secrets
  kubectl exec -i openbao-0 -n openbao -- bao policy write csi - <<EOF
path "secrets/data/*" {
 capabilities = ["read"]
}
EOF

  # create authentication role
  kubectl exec -i openbao-0 -n openbao -- bao write auth/kubernetes/role/csi \
    bound_service_account_names=default \
    bound_service_account_namespaces=default,test-ns,negative-test-ns \
    policies=csi \
    ttl=20m
}

@test "deploy openbao secretproviderclass crd" {
  kubectl apply -f $BATS_TESTS_DIR/openbao_v1_secretproviderclass.yaml
  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/openbao-foo -o yaml | grep openbao"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
  kubectl apply -f $BATS_TESTS_DIR/pod-openbao-inline-volume-secretproviderclass.yaml
  # wait for pod to be running
  kubectl wait --for=condition=Ready --timeout=60s pod/secrets-store-inline

  run kubectl get pod/secrets-store-inline
  assert_success
}

@test "CSI inline volume test with pod portability - read openbao secret from pod" {
  result=$(kubectl exec secrets-store-inline -- cat /mnt/secrets-store/foo)
  [[ "$result" == "openbao-foo" ]]

  result=$(kubectl exec secrets-store-inline -- cat /mnt/secrets-store/bar)
  [[ "$result" == "openbao-bar" ]]
}

@test "CSI inline volume test with pod portability - rotation succeeds" {
  # seed first value
  kubectl exec openbao-0 -n openbao -- bao kv put secrets/rotation foo=start

  # deploy pod
  kubectl apply -f $BATS_TESTS_DIR/pod-openbao-rotation.yaml
  kubectl wait --for=condition=Ready --timeout=60s pod/secrets-store-rotation

  run kubectl get pod/secrets-store-rotation
  assert_success

  # verify starting value
  result=$(kubectl exec secrets-store-rotation -- cat /mnt/secrets-store/foo)
  [[ "$result" == "start" ]]

  # update the secret value
  kubectl exec openbao-0 -n openbao -- bao kv put secrets/rotation foo=rotated

  sleep 130 # wait for rotation to occur

  # verify rotated value
  result=$(kubectl exec secrets-store-rotation -- cat /mnt/secrets-store/foo)
  [[ "$result" == "rotated" ]]
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

@test "Sync with K8s secrets - create deployment" {
  kubectl apply -f $BATS_TESTS_DIR/openbao_synck8s_v1_secretproviderclass.yaml
  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/openbao-foo-sync -o yaml | grep openbao"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl apply -f $BATS_TESTS_DIR/deployment-synck8s.yaml
  assert_success

  run kubectl apply -f $BATS_TESTS_DIR/deployment-two-synck8s.yaml
  assert_success

  kubectl wait --for=condition=Ready --timeout=120s pod -l app=busybox
}

@test "Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences with multiple owners" {
  POD=$(kubectl get pod -l app=busybox -o jsonpath="{.items[0].metadata.name}")
  result=$(kubectl exec $POD -- cat /mnt/secrets-store/foo)
  [[ "$result" == "openbao-foo" ]]

  result=$(kubectl exec $POD -- cat /mnt/secrets-store/bar)
  [[ "$result" == "openbao-bar" ]]

  result=$(kubectl exec $POD -- cat /mnt/secrets-store/nested/foo)
  [[ "$result" == "openbao-foo" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.data.pwd}" | base64 -d)
  [[ "$result" == "openbao-foo" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.data.nested}" | base64 -d)
  [[ "$result" == "openbao-foo" ]]

  result=$(kubectl exec $POD -- printenv | grep SECRET_USERNAME | awk -F"=" '{ print $2 }' | tr -d '\r\n')
  [[ "$result" == "openbao-bar" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.metadata.labels.environment}")
  [[ "${result//$'\r'/}" == "${LABEL_VALUE}" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.metadata.annotations.kubed\.appscode\.com\/sync}")
  [[ "${result//$'\r'/}" == "${ANNOTATION_VALUE}" ]]

  result=$(kubectl get secret foosecret -o jsonpath="{.metadata.labels.secrets-store\.csi\.k8s\.io/managed}")
  [[ "${result//$'\r'/}" == "true" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret default 2"
  assert_success
}

@test "Sync with K8s secrets - delete deployment, check secret is deleted" {
  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s.yaml
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret default 1"
  assert_success

  run kubectl delete -f $BATS_TESTS_DIR/deployment-two-synck8s.yaml
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted foosecret default"
  assert_success

  run kubectl delete -f $BATS_TESTS_DIR/openbao_synck8s_v1_secretproviderclass.yaml
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - create deployment" {
  kubectl create ns test-ns

  kubectl apply -f $BATS_TESTS_DIR/openbao_v1_secretproviderclass_ns.yaml
  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/openbao-foo-sync -o yaml | grep openbao"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/openbao-foo-sync -n test-ns -o yaml | grep openbao"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  kubectl apply -n test-ns -f $BATS_TESTS_DIR/deployment-synck8s.yaml

  kubectl wait --for=condition=Ready --timeout=90s pod -l app=busybox -n test-ns
}

@test "Test Namespaced scope SecretProviderClass - Sync with K8s secrets - read secret from pod, read K8s secret, read env var, check secret ownerReferences" {
  POD=$(kubectl get pod -l app=busybox -n test-ns -o jsonpath="{.items[0].metadata.name}")
  result=$(kubectl exec -n test-ns $POD -- cat /mnt/secrets-store/foo)
  [[ "$result" == "openbao-foo" ]]

  result=$(kubectl exec -n test-ns $POD -- cat /mnt/secrets-store/bar)
  [[ "$result" == "openbao-bar" ]]

  result=$(kubectl get secret foosecret -n test-ns -o jsonpath="{.data.pwd}" | base64 -d)
  [[ "$result" == "openbao-foo" ]]

  result=$(kubectl exec -n test-ns $POD -- printenv | grep SECRET_USERNAME | awk -F"=" '{ print $2 }' | tr -d '\r\n')
  [[ "$result" == "openbao-bar" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret test-ns 1"
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - Sync with K8s secrets - delete deployment, check secret deleted" {
  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s.yaml -n test-ns
  assert_success

  run wait_for_process $WAIT_TIME $SLEEP_TIME "check_secret_deleted foosecret test-ns"
  assert_success
}

@test "Test Namespaced scope SecretProviderClass - Should fail when no secret provider class in same namespace" {
  kubectl create ns negative-test-ns

  kubectl apply -n negative-test-ns -f $BATS_TESTS_DIR/deployment-synck8s.yaml

  POD=$(kubectl get pod -l app=busybox -n negative-test-ns -o jsonpath="{.items[0].metadata.name}")
  cmd="kubectl describe pod $POD -n negative-test-ns | grep 'FailedMount.*failed to get secretproviderclass negative-test-ns/openbao-foo-sync.*not found'"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  run kubectl delete -f $BATS_TESTS_DIR/deployment-synck8s.yaml -n negative-test-ns
  assert_success

  run kubectl delete ns negative-test-ns
  assert_success
}

@test "deploy multiple openbao secretproviderclass crd" {
  kubectl apply -f $BATS_TESTS_DIR/openbao_v1_multiple_secretproviderclass.yaml

  cmd="kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/openbao-foo-sync-0 -o yaml | grep openbao-foo-sync-0"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/openbao-foo-sync-1 -o yaml | grep openbao-foo-sync-1"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "deploy pod with multiple secret provider class" {
  kubectl apply -f $BATS_TESTS_DIR/pod-openbao-inline-volume-multiple-spc.yaml
  kubectl wait --for=condition=Ready --timeout=90s pod/secrets-store-inline-multiple-crd

  run kubectl get pod/secrets-store-inline-multiple-crd
  assert_success
}

@test "CSI inline volume test with multiple secret provider class" {
  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-0/foo)
  [[ "$result" == "openbao-foo" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-0/bar)
  [[ "$result" == "openbao-bar" ]]

  result=$(kubectl get secret foosecret-0 -o jsonpath="{.data.pwd}" | base64 -d)
  [[ "$result" == "openbao-foo" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- printenv | grep SECRET_USERNAME_0 | awk -F"=" '{ print $2 }' | tr -d '\r\n')
  [[ "$result" == "openbao-bar" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret-0 default 1"
  assert_success

  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-1/foo)
  [[ "$result" == "openbao-foo" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- cat /mnt/secrets-store-1/bar)
  [[ "$result" == "openbao-bar" ]]

  result=$(kubectl get secret foosecret-1 -o jsonpath="{.data.pwd}" | base64 -d)
  [[ "$result" == "openbao-foo" ]]

  result=$(kubectl exec secrets-store-inline-multiple-crd -- printenv | grep SECRET_USERNAME_1 | awk -F"=" '{ print $2 }' | tr -d '\r\n')
  [[ "$result" == "openbao-bar" ]]

  run wait_for_process $WAIT_TIME $SLEEP_TIME "compare_owner_count foosecret-1 default 1"
  assert_success
}

teardown_file() {
  archive_info || true
}
