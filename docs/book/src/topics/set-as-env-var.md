# Set as ENV var

<details>
<summary>Examples</summary>

- `SecretProviderClass`

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1alpha1
kind: SecretProviderClass
metadata:
  name: azure-sync
spec:
  provider: azure
  secretObjects:                                 # [OPTIONAL] SecretObject defines the desired state of synced K8s secret objects
  - secretName: foosecret
    type: Opaque
    labels:                                   
      environment: "test"
    data: 
    - objectName: secretalias                    # name of the mounted content to sync. this could be the object name or object alias 
      key: username
  parameters:
    usePodIdentity: "false"                      
    keyvaultName: "$KEYVAULT_NAME"               # the name of the KeyVault
    objects: |
      array:
        - |
          objectName: $SECRET_NAME
          objectType: secret                     # object types: secret, key or cert
          objectAlias: secretalias
          objectVersion: $SECRET_VERSION         # [OPTIONAL] object versions, default to latest if empty
        - |
          objectName: $KEY_NAME
          objectType: key
          objectVersion: $KEY_VERSION
    tenantId: "tid"                             # the tenant ID of the KeyVault
``` 

- `Pod` yaml

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: secrets-store-inline
spec:
  containers:
    - name: busybox
      image: k8s.gcr.io/e2e-test-images/busybox:1.29
      command:
      - "/bin/sleep"
      - "10000"
      volumeMounts:
      - name: secrets-store01-inline
        mountPath: "/mnt/secrets-store"
        readOnly: true
        env:
        - name: SECRET_USERNAME
          valueFrom:
            secretKeyRef:
              name: foosecret
              key: username
  volumes:
    - name: secrets-store01-inline
      csi:
        driver: secrets-store.csi.k8s.io
        readOnly: true
        volumeAttributes:
          secretProviderClass: "azure-sync"
```

</details>

Once the [secret is created](./sync-as-kubernetes-secret.md), you may wish to set an ENV VAR in your deployment to reference the new Kubernetes secret.

```yaml
spec:
  containers:
  - image: k8s.gcr.io/e2e-test-images/busybox:1.29
    name: busybox
    command:
    - "/bin/sleep"
    - "10000"
    env:
    - name: SECRET_USERNAME
      valueFrom:
        secretKeyRef:
          name: foosecret
          key: username
```
