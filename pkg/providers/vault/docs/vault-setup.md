# Setting Up a Development Vault Cluster

This guide showcases how to create a development HashiCorp Vault cluster in Kubernetes.
It also shows how to configure the Kubernetes [auth method](https://www.vaultproject.io/docs/auth/kubernetes.html) to enable Vault to make API calls to Kubernetes. This allows applications/users to login to Vault using the Kubernetes
service account tokens. As part of the guide we will also store a secret as an example to showcase the Vault provider for Secret Store CSI driver.

## Deploy Vault Pod

```bash
kubectl apply -f pkg/providers/vault/examples/vault.yaml
```

This will create a Kubernetes pod running Vault in ["dev" mode](https://www.vaultproject.io/docs/concepts/dev-server.html).

## Configure Kubernetes Auth Method

Port forward to the Vault pod

```bash
VAULT_POD=$(kubectl get pod -l app=vault -o jsonpath="{.items[0].metadata.name}")
kubectl port-forward ${VAULT_POD} 8200:8200 &
```

Export Vault address and token

```bash
export VAULT_ADDR="http://127.0.0.1:8200"
export VAULT_TOKEN="root"
```

Note: *This is an example Vault cluster running in "dev" mode. This is not recommended
for a production deployment for Vault.*

Create a Kubernetes service account for Vault

```bash
kubectl create serviceaccount vault-auth
```

Create a cluster role binding that uses the service account

```yaml
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
```

Configure variables for enabling Kubernetes auth method

```bash
CLUSTER_NAME="$(kubectl config view --raw \
  -o go-template="{{ range .contexts }}{{ if eq .name \"$(kubectl config current-context)\" }}{{ index .context \"cluster\" }}{{ end }}{{ end }}")"

SECRET_NAME="$(kubectl get serviceaccount vault-auth \
  -o go-template='{{ (index .secrets 0).name }}')"

TR_ACCOUNT_TOKEN="$(kubectl get secret ${SECRET_NAME} \
  -o go-template='{{ .data.token }}' | base64 --decode)"

K8S_HOST="https://$(kubectl get svc kubernetes -o go-template="{{ .spec.clusterIP }}")"

# if you have embedded client cert, use the commands below.
K8S_CACERT="$(kubectl config view --raw \
  -o go-template="{{ range .clusters }}{{ if eq .name \"${CLUSTER_NAME}\" }}{{ index .cluster \"certificate-authority-data\" }}{{ end }}{{ end }}" | base64 --decode)"

# if you haven't embedded client cert, use the commands below.
K8S_CACERT="$(cat $(kubectl config view --raw \
  -o go-template="{{ range .clusters }}{{ if eq .name \"${CLUSTER_NAME}\" }}{{ index .cluster \"certificate-authority\" }}{{ end }}{{ end }}"))"
```

Enable Kubernetes Auth Method

```bash
vault auth enable kubernetes
```

Configure Kubernetes Auth Method

```bash
vault write auth/kubernetes/config \
  kubernetes_host="${K8S_HOST}" \
  kubernetes_ca_cert="${K8S_CACERT}" \
  token_reviewer_jwt="${TR_ACCOUNT_TOKEN}"
```

You have now successfully enabled and configured the Kubernetes Auth Method in Vault!

You can also follow the [guide](https://www.vaultproject.io/docs/auth/kubernetes.html) on the Vault website to configure Kubernetes auth method.

## Vault Provider for Secret Store CSI Driver

We will now create an example policy, role, and a secret in Vault to showcase the Vault provider for
Secret Store CSI driver.

Create an example Vault policy

```bash
echo 'path "secret/data/foo" {
  capabilities = ["read", "list"]
}

path "sys/renew/*" {
  capabilities = ["update"]
}' | vault policy write example-readonly -
```

Create an example Vault role

```bash
vault write auth/kubernetes/role/example-role \
  bound_service_account_names=csi-driver-registrar \
  bound_service_account_namespaces=<SECRETS-STORE-CSI-DRIVER NAMESPACE> \
  policies=default,example-readonly \
  ttl=20m
```

The above role has the `csi-driver-registrar` Kubernetes service account bound to it. This means
that any Kubernetes pod that has the service account token can use it to login to Vault.

Write an example secret in the Vault Key-Value store

```bash
vault kv put secret/foo bar=hello
```



