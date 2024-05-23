#!/usr/bin/env bats

load helpers

BATS_TESTS_DIR=test/bats/tests/conjur
WAIT_TIME=180
SLEEP_TIME=1

CONJUR_NAMESPACE=conjur
CONJUR_DATA_KEY="$(openssl rand -base64 32)"
CONJUR_ACCOUNT=default
CONJUR_URL=conjur-conjur-oss.conjur.svc.cluster.local

EXPECTED_USERNAME="some_user"
EXPECTED_PASSWORD="SecretPassword1234!"

@test "install conjur provider" { 
  # Update Helm repos
  helm repo add cyberark https://cyberark.github.io/helm-charts || true
  helm repo update

  # Install Conjur
  helm install conjur cyberark/conjur-oss --namespace=$CONJUR_NAMESPACE --wait --timeout ${WAIT_TIME}s --create-namespace \
    --set dataKey=$CONJUR_DATA_KEY \
    --set authenticators="authn\,authn-jwt/kube" \
    --set service.external.enabled=false

  # Install Conjur CSI Provider
  helm install conjur-csi-provider cyberark/conjur-k8s-csi-provider --wait --timeout ${WAIT_TIME}s --namespace=kube-system \
    --set providerServer.image.tag=0.1.0
} 

@test "setup conjur" { 
  # Create Conjur account and store admin API key
  admin_api_key="$(kubectl exec deployment/conjur-conjur-oss \
    --namespace=$CONJUR_NAMESPACE \
    --container=conjur-oss \
    -- conjurctl account create $CONJUR_ACCOUNT | grep API | awk '{print $5}')"

  # Create a Conjur CLI pod
  kubectl run conjur-cli-pod --image=cyberark/conjur-cli:8 --namespace=$CONJUR_NAMESPACE --command -- sleep infinity
  kubectl wait --for=condition=ready --timeout=${WAIT_TIME}s --namespace=$CONJUR_NAMESPACE pod/conjur-cli-pod

  # Get values required by authn-jwt authenticator
  ISSUER=$(kubectl get --raw /.well-known/openid-configuration | jq -r .issuer)
  JWKS='{"type":"jwks","value":'$(kubectl get --raw /openid/v1/jwks)'}'

  # Copy policy files into CLI container and configure Conjur
  kubectl -n "${CONJUR_NAMESPACE}" cp $BATS_TESTS_DIR/policy conjur-cli-pod:/policy -c conjur-cli-pod
  kubectl -n "${CONJUR_NAMESPACE}" exec conjur-cli-pod -- /bin/sh -c "
  set -x
  # Initialise CLI and login
  echo yes | conjur init -u 'https://$CONJUR_URL' -a '$CONJUR_ACCOUNT' --self-signed
  conjur login -i admin -p $admin_api_key

  # Apply policy
  conjur policy replace -b root -f /policy/host.yaml
  conjur policy load -b root -f /policy/authn-jwt.yaml
  conjur policy load -b root -f /policy/variables.yaml
  # Set test secret values
  conjur variable set -i db-credentials/username -v '$EXPECTED_USERNAME'
  conjur variable set -i db-credentials/password -v '$EXPECTED_PASSWORD'
  # Set variable values on authenticator
  conjur variable set -i conjur/authn-jwt/kube/public-keys -v '$JWKS'
  conjur variable set -i conjur/authn-jwt/kube/issuer -v '$ISSUER'
  "
}

@test "deploy conjur secretproviderclass crd" {
  CONJUR_POD=$(kubectl get pods --namespace=$CONJUR_NAMESPACE -l "app=conjur-oss" -o=jsonpath='{.items[0].metadata.name}')
  export CONJUR_SSL_CERT=$(kubectl exec --namespace=$CONJUR_NAMESPACE  -c conjur-oss $CONJUR_POD  -- sh -c "openssl s_client -showcerts -connect $CONJUR_URL:443 </dev/null 2>/dev/null | sed -n '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p'")
  envsubst < $BATS_TESTS_DIR/conjur_v1_secretproviderclass.yaml | sed '/^ *-----BEGIN CERTIFICATE-----/,$s/^/      /' | kubectl apply -f -

  kubectl wait --for condition=established --timeout=60s crd/secretproviderclasses.secrets-store.csi.x-k8s.io

  cmd="kubectl get secretproviderclasses.secrets-store.csi.x-k8s.io/conjur -o yaml | grep conjur"
  wait_for_process $WAIT_TIME $SLEEP_TIME "$cmd"
}

@test "CSI inline volume test with pod portability" {
  kubectl apply -f $BATS_TESTS_DIR/pod-secrets-store-inline-volume-crd.yaml

  kubectl wait --for=condition=Ready --timeout=60s pod/secrets-store-inline-crd

  run kubectl get pod/secrets-store-inline-crd
  assert_success
}

@test "CSI inline volume test with pod portability - read conjur secret from pod" {
  result=$(kubectl exec secrets-store-inline-crd -- cat /mnt/secrets-store/relative/path/username)
  [[ "${result}" == "$EXPECTED_USERNAME" ]]

  result=$(kubectl exec secrets-store-inline-crd -- cat /mnt/secrets-store/relative/path/password)
  [[ "${result}" == "$EXPECTED_PASSWORD" ]]
}

@test "CSI inline volume test with pod portability - rotation succeeds" {
  # Set initial value
  kubectl -n conjur exec conjur-cli-pod -- conjur variable set -i db-credentials/username -v initial_value

  # Deploy pod
  CONJUR_POD=$(kubectl get pods --namespace=$CONJUR_NAMESPACE -l "app=conjur-oss" -o=jsonpath='{.items[0].metadata.name}')
  export CONJUR_SSL_CERT=$(kubectl exec --namespace=$CONJUR_NAMESPACE -c conjur-oss $CONJUR_POD  -- sh -c "openssl s_client -showcerts -connect $CONJUR_URL:443 </dev/null 2>/dev/null | sed -n '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p'")
  envsubst < $BATS_TESTS_DIR/pod-conjur-rotation.yaml | sed '/^ *-----BEGIN CERTIFICATE-----/,$s/^/      /' | kubectl apply -f -
  kubectl wait --for=condition=Ready --timeout=60s pod/secrets-store-inline-rotation

  run kubectl get pod/secrets-store-inline-rotation
  assert_success

  # Verify initial value
  result=$(kubectl exec secrets-store-inline-rotation -- cat /mnt/secrets-store/relative/path/username)
  [[ "${result}" == "initial_value" ]]

  # Update the secret value and wait for rotation interval
  kubectl -n conjur exec conjur-cli-pod -- conjur variable set -i db-credentials/username -v rotated_value
  sleep 40

  # Verify rotated value
  result=$(kubectl exec secrets-store-inline-rotation -- cat /mnt/secrets-store/relative/path/username)
  [[ "${result}" == "rotated_value" ]]
}

@test "CSI inline volume test with pod portability - unmount succeeds" {
  # On Linux a failure to unmount the tmpfs will block the pod from being deleted.
  run kubectl delete pod secrets-store-inline-crd
  assert_success

  run kubectl wait --for=delete --timeout=${WAIT_TIME}s pod/secrets-store-inline-crd
   
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
  archive_provider "app=csi-secrets-store-provider-conjur" || true
  archive_info || true
}
