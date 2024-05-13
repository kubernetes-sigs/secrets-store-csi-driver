# Deploying the SecretSync Controller
You can deploy the SecretSync Controller using Helm. This guide provides instructions for deploying the SecretSync Controller using Helm.

You can use the following commands to deploy the SecretSync Controller using Helm:
```sh
helm install -f values <path_to_values.yaml> secret-sync-controller charts/secretsync
```

# Configuration and Parameters
You can customize the installation by modifying values in the values.yaml file or by passing parameters to the helm install command using the --set key=value[,key=value] argument.

| Parameter Name                          | Description                                                                                                         | Default Value                                                 |
|-----------------------------------------|---------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------|
| `providerContainer`                 | The container for the Secret Sync Provider                                                   | `[- name: provider-aws-installer ...]`                                                        |
| `controllerName`                        | The name of the SecretSync Controller.                                                                              | `secret-sync-controller-manager`                   |
| `namespace`                             | The namespace to deploy the chart to.                                                                               | `secret-sync-controller-system`                               |
| `tokenRequestAudience`                  | The audience for the token request.                                                                                 | `[]`                                  |
| `rotationPollInterval`         | How quickly the SecretSync Controller checks or updates the secret it is managing.                                  | `21600s` (6 hours)                                             |
| `logVerbosity`                          | The log level.                                                                                                      | `5`                                                           |
| `validatingAdmissionPolicies.applyPolicies` | Determines whether the SecretSync Controller should apply policies.                                             | `true`                                                        |
| `validatingAdmissionPolicies.allowedSecretTypes` | The types of secrets that the SecretSync Controller should allow.                                          | `["Opaque", "kubernetes.io/basic-auth", "bootstrap.kubernetes.io/token", "kubernetes.io/dockerconfigjson", "kubernetes.io/dockercfg", "kubernetes.io/ssh-auth", "kubernetes.io/tls"]` |
| `validatingAdmissionPolicies.deniedSecretTypes`| The types of secrets that the SecretSync Controller should deny.                                             | `["kubernetes.io/service-account-token"]`                     |
| `image.repository`                      | The image repository of the SecretSync Controller.                                                                  | `aramase/secrets-sync-controller:v0.0.1`         |
| `image.pullPolicy`                      | Image pull policy.                                                                                                  | `IfNotPresent`                                                |
| `image.tag`                             | The specific image tag to use. Overrides the image tag whose default is the chart's `appVersion`.                   | `""`                                                          |
| `imagePullSecrets`                      | Array of image pull secrets for accessing private registries.                                                       | `[{"name": "regcred"}]`                                       |
| `nameOverride`                          | A string to partially override `secretsync.fullname` template (will maintain the release name).                     | `""`                                                          |
| `fullnameOverride`                      | A string to fully override `secretsync.fullname` template.                                                          | `""`                                                          |
| `securityContext`                       | Security context for the SecretSync Controller.                                                                     | `{ allowPrivilegeEscalation: false, capabilities: { drop: [ALL] } }` |
| `resources`                             | The resource request/limits for the SecretSync Controller image.                                                    | `limits: 500m CPU, 128Mi; requests: 10m CPU, 64Mi`            |
| `podAnnotations`                        | Annotations to be added to pods.                                                                                    | `{ kubectl.kubernetes.io/default-container: "manager" }`      |
| `podLabels`                             | Labels to be added to pods.                                                                                         | `{ control-plane: "controller-manager", secrets-store.io/system: "true" }` |
| `nodeSelector`                          | Node labels for pod assignment.                                                                                     | `{ kubernetes.io/os: "linux" }`                               |
| `tolerations`                           | Tolerations for pod assignment.                                                                                     | `[{ operator: "Exists" }]`                                    |
| `affinity`                              | Affinity settings for pod assignment.                                                                               | `key: type; operator: NotIn; values: [virtual-kubelet]`       |


These parameters offer flexibility in configuring and deploying the SecretSync Controller according to specific requirements in your Kubernetes environment. Remember to replace values appropriately or use the --set flag when installing the chart via Helm.
