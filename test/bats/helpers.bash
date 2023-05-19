#!/bin/bash

assert_success() {
  if [[ "$status" != 0 ]]; then
    echo "expected: 0"
    echo "actual: $status"
    echo "output: $output"
    return 1
  fi
}

assert_failure() {
  if [[ "$status" == 0 ]]; then
    echo "expected: non-zero exit code"
    echo "actual: $status"
    echo "output: $output"
    return 1
  fi
}

assert_equal() {
  if [[ "$1" != "$2" ]]; then
    echo "expected: $1"
    echo "actual: $2"
    return 1
  fi
}

assert_not_equal() {
  if [[ "$1" == "$2" ]]; then
    echo "unexpected: $1"
    echo "actual: $2"
    return 1
  fi
}

assert_match() {
  if [[ ! "$2" =~ $1 ]]; then
    echo "expected: $1"
    echo "actual: $2"
    return 1
  fi
}

assert_not_match() {
  if [[ "$2" =~ $1 ]]; then
    echo "expected: $1"
    echo "actual: $2"
    return 1
  fi
}

wait_for_process(){
  wait_time="$1"
  sleep_time="$2"
  cmd="$3"
  while [ "$wait_time" -gt 0 ]; do
    if eval "$cmd"; then
      return 0
    else
      sleep "$sleep_time"
      wait_time=$((wait_time-sleep_time))
    fi
  done
  return 1
}

compare_owner_count() {
  secret="$1"
  namespace="$2"
  ownercount="$3"

  [[ "$(kubectl get secret ${secret} -n ${namespace} -o json | jq '.metadata.ownerReferences | length')" -eq $ownercount ]]
}

check_secret_deleted() {
  secret="$1"
  namespace="$2"

  result=$(kubectl get secret -n ${namespace} | grep "^${secret}$" | wc -l)
  [[ "$result" -eq 0 ]]
}

# Usage:
#
# archive_provider "<pod label selector>"
#
# provider pod must be in kube-system
archive_provider() {
  if [[ -z "${ARTIFACTS}" ]]; then
    return 0
  fi

  FILE_PREFIX=$(date +"%FT%H%M%S")

  kubectl logs -l $1 --tail -1 -n kube-system > ${ARTIFACTS}/${FILE_PREFIX}-provider.logs
}

archive_info() {
  if [[ -z "${ARTIFACTS}" ]]; then
    return 0
  fi

  LOGS_DIR=${ARTIFACTS}/$(date +"%FT%H%M%S")
  mkdir -p "${LOGS_DIR}"

  # print all pod information
  kubectl get pods -A -o json > ${LOGS_DIR}/pods.json

  # print detailed pod information
  kubectl describe pods --all-namespaces > ${LOGS_DIR}/pods-describe.txt

  # print logs from the CSI Driver
  #
  # assumes driver is installed with helm into the `kube-system` namespace which
  # sets the `app` selector to `secrets-store-csi-driver`.
  #
  # Note: the yaml deployment would require `app=csi-secrets-store`
  kubectl logs -l app=secrets-store-csi-driver  --tail -1 -c secrets-store -n kube-system > ${LOGS_DIR}/secrets-store.log
  kubectl logs -l app=secrets-store-csi-driver  --tail -1 -c node-driver-registrar -n kube-system > ${LOGS_DIR}/node-driver-registrar.log
  kubectl logs -l app=secrets-store-csi-driver  --tail -1 -c liveness-probe -n kube-system > ${LOGS_DIR}/liveness-probe.log

  # print client and server version information
  kubectl version > ${LOGS_DIR}/kubectl-version.txt

  # print generic cluster information
  kubectl cluster-info dump > ${LOGS_DIR}/cluster-info.txt

  # collect metrics
  local curl_pod_name=curl-$(openssl rand -hex 5)
  kubectl run ${curl_pod_name} -n default --image=curlimages/curl:7.75.0 --labels="test=metrics_test" --overrides='{"spec": { "nodeSelector": {"kubernetes.io/os": "linux"}}}' -- tail -f /dev/null
  kubectl wait --for=condition=Ready --timeout=60s -n default pod ${curl_pod_name}

  for pod_ip in $(kubectl get pod -n kube-system -l app=secrets-store-csi-driver -o jsonpath="{.items[*].status.podIP}")
  do
    kubectl exec -n default ${curl_pod_name} -- curl -s http://${pod_ip}:8095/metrics > ${LOGS_DIR}/${pod_ip}.metrics
  done

  kubectl delete pod -n default ${curl_pod_name}
}

# get_secrets_store_api_version returns the API version of the secrets-store API
get_secrets_store_api_version() {
  local api_version=$(kubectl api-resources --api-group='secrets-store.csi.x-k8s.io' --no-headers=true | awk '{ print $2 }' | uniq)
  echo "${api_version}"
}

# log the secrets-store API version
log_secrets_store_api_version() {
  echo "Testing secrets-store API version $API_VERSION" >&3
}

get_token_requests_audience() {
  local token_requests_audience=$(kubectl get csidriver secrets-store.csi.k8s.io -o go-template --template='{{range $i, $v := .spec.tokenRequests}}{{if $i}},{{end}}{{printf "%s" .audience}}{{end}}')
  echo "${token_requests_audience}"
}

log_token_requests_audience() {
  echo "Testing token requests audience $VALIDATE_TOKENS_AUDIENCE" >&3
}
