# Sync All Secrets to K8s

<details>
<summary>Opaque Examples</summary>

- In this example, we can expect the secrets that are mounted on the paths `/mnt/secrets-store/username` and `/mnt/secrets-store/password` to be synced to K8s individually.
- The value of `objectName` is the name of the K8s secret


```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: vault-opaque
spec:
  provider: vault
  parameters:
    roleName: "csi"
    vaultAddress: "http://vault.vault:8200"
    objects: |
      - secretPath: "secret/data/db-creds"
        objectName: "username"
        secretKey: "username"
      - secretPath: "secret/data/db-creds"
        objectName: "password"
        secretKey: "password"
  syncOptions:
    syncAll: true
    type: Opaque
---
kind: ServiceAccount
apiVersion: v1
metadata:
  name: opaque-sa
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: busybox-deployment-opaque
  labels:
    app: busybox
spec:
  replicas: 1
  selector:
    matchLabels:
      app: busybox
  template:
    metadata:
      labels:
        app: busybox
    spec:
      terminationGracePeriodSeconds: 0
      serviceAccountName: opaque-sa
      containers:
        - image: k8s.gcr.io/e2e-test-images/busybox:1.29
          name: busybox
          imagePullPolicy: IfNotPresent
          command:
            - "/bin/sleep"
            - "10000"
          env:
            - name: DB_USER
              valueFrom:
                secretKeyRef:
                  name: username
                  key: username
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: password
                  key: password
          volumeMounts:
            - name: secrets-store-inline
              mountPath: "/mnt/secrets-store"
              readOnly: true
      volumes:
        - name: secrets-store-inline
          csi:
            driver: secrets-store.csi.k8s.io
            readOnly: true
            volumeAttributes:
              secretProviderClass: "vault-opaque"

```

- In this example, we are nesting some secrets further into the filesystem. Since K8s secrets must have unique names, the name of the synced secret will be a hyphen-separated version of the `objectName` value.
- In this case, we can expect the following secrets in K8s
1. username
2. password
3. nested-username
4. nested-password

- You will also notice that we have one item in the `.spec.secretObjects` field. With `Opaque` type secrets, we can sync all mounted secrets into a single K8s secret. **This does not work with other secret types**
- In this example, we can expect a K8s secret named `db-secret` with the following keys
1. username
2. password
3. nested-username
4. nested-password

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: vault-opaque
spec:
  provider: vault
  parameters:
    roleName: "csi"
    vaultAddress: "http://vault.vault:8200"
    objects: |
      - secretPath: "secret/data/db-creds"
        objectName: "username"
        secretKey: "username"
      - secretPath: "secret/data/db-creds"
        objectName: "password"
        secretKey: "password"
      - secretPath: "secret/data/db-creds"
        objectName: "nested/username"
        secretKey: "username"
      - secretPath: "secret/data/db-creds"
        objectName: "nested/password"
        secretKey: "password"
  syncOptions:
    type: Opaque
    syncAll: true
  secretObjects:
    - secretName: db-secret
      type: Opaque
      syncAll: true
---
kind: ServiceAccount
apiVersion: v1
metadata:
  name: opaque-sa
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: busybox-deployment-opaque
  labels:
    app: busybox
spec:
  replicas: 1
  selector:
    matchLabels:
      app: busybox
  template:
    metadata:
      labels:
        app: busybox
    spec:
      terminationGracePeriodSeconds: 0
      serviceAccountName: opaque-sa
      containers:
        - image: k8s.gcr.io/e2e-test-images/busybox:1.29
          name: busybox
          imagePullPolicy: IfNotPresent
          command:
            - "/bin/sleep"
            - "10000"
          env:
            - name: DB_USER
              valueFrom:
                secretKeyRef:
                  name: username
                  key: username
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: password
                  key: password
            - name: DB_NESTED_USER
              valueFrom:
                secretKeyRef:
                  name: nested-username
                  key: username
            - name: DB_NESTED_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: nested-password
                  key: password
            - name: DB_SECRET_USER
              valueFrom:
                secretKeyRef:
                  name: db-secret
                  key: username
            - name: DB_SECRET_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: db-secret
                  key: password
            - name: DB_SECRET_NESTED_USER
              valueFrom:
                secretKeyRef:
                  name: db-secret
                  key: username
            - name: DB_SECRET_NESTED_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: db-secret
                  key: password
          volumeMounts:
            - name: secrets-store-inline
              mountPath: "/mnt/secrets-store"
              readOnly: true
      volumes:
        - name: secrets-store-inline
          csi:
            driver: secrets-store.csi.k8s.io
            readOnly: true
            volumeAttributes:
              secretProviderClass: "vault-opaque"

```

</details>

<details>
<summary>TLS Examples</summary>

- For TLS secrets, you need to mount the certificate and private key on a single mount as shown in the example below.
- The driver will separate the two values and assign them to the `tls.crt` and `tls.key` keys respectively.

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: vault-tls
spec:
  provider: vault
  parameters:
    roleName: "csi"
    vaultAddress: "http://vault.vault:8200"
    objects: |
      - secretPath: "secret/data/certs"
        objectName: "cert1"
        secretKey: "cert1"
      - secretPath: "secret/data/certs"
        objectName: "cert2"
        secretKey: "cert2"
  syncOptions:
    syncAll: true
    type: kubernetes.io/tls
---
kind: ServiceAccount
apiVersion: v1
metadata:
  name: tls-sa
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: busybox-deployment-tls
  labels:
    app: busybox-tls
spec:
  replicas: 1
  selector:
    matchLabels:
      app: busybox-tls
  template:
    metadata:
      labels:
        app: busybox-tls
    spec:
      terminationGracePeriodSeconds: 0
      serviceAccountName: tls-sa
      containers:
        - image: k8s.gcr.io/e2e-test-images/busybox:1.29
          name: busybox-tls
          imagePullPolicy: IfNotPresent
          command:
            - "/bin/sleep"
            - "10000"
          env:
            - name: PRIVATE_KEY_1
              valueFrom:
                secretKeyRef:
                  name: cert1
                  key: tls.key
            - name: CERTIFICATE_1
              valueFrom:
                secretKeyRef:
                  name: cert1
                  key: tls.crt
            - name: PRIVATE_KEY_2
              valueFrom:
                secretKeyRef:
                  name: cert2
                  key: tls.key
            - name: CERTIFICATE_2
              valueFrom:
                secretKeyRef:
                  name: cert2
                  key: tls.crt
          volumeMounts:
            - name: secrets-store-inline
              mountPath: "/mnt/secrets-store"
              readOnly: true
      volumes:
        - name: secrets-store-inline
          csi:
            driver: secrets-store.csi.k8s.io
            readOnly: true
            volumeAttributes:
              secretProviderClass: "vault-tls"
```

```txt
-----BEGIN CERTIFICATE-----
MIIDOTCCAiGgAwIBAgIJAP0J5Z7N0Y5fMA0GCSqGSIb3DQEBCwUAMDMxFzAVBgNV
BAMMDmRlbW8uYXp1cmUuY29tMRgwFgYDVQQKDA9ha3MtaW5ncmVzcy10bHMwHhcN
MjAwNDE1MDQyMzQ2WhcNMjEwNDE1MDQyMzQ2WjAzMRcwFQYDVQQDDA5kZW1vLmF6
dXJlLmNvbTEYMBYGA1UECgwPYWtzLWluZ3Jlc3MtdGxzMIIBIjANBgkqhkiG9w0B
AQEFAAOCAQ8AMIIBCgKCAQEAyS3Zky3n8JlLBxPLzgUpKZYxvzRadeWLmWVbK9by
o08S0Ss8Jao7Ay1wHtnLbn52rzCX6IX1sAe1TAT755Gk7JtLMkshtj6F8BNeelEy
E1gsBE5ntY5vyLTm/jZUIKz2Z9TLnqvQTmp6gJ68BKJ1NobnsHiAcKc6hI7kmY9C
oshmAi5qiKYBgzv/thji0093vtVSa9iwHhQp+AEIMhkvM5ZZkiU5eE6MT9SBEcVW
KmWF28UsB04daYwS2MKJ5l6d4n0LUdAG0FBt1lCoT9rwUDj9l3Mqmi953gw26LUr
NrYnM/8N2jl7Cuyw5alIWaUDrt5i+pu8wdWfzVk+fO7x8QIDAQABo1AwTjAdBgNV
HQ4EFgQUwFBbR014McETdrGGklpEQcl71Q0wHwYDVR0jBBgwFoAUwFBbR014McET
drGGklpEQcl71Q0wDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEATgTy
gg1Q6ISSekiBCe12dqUTMFQh9GKpfYWKRbMtjOjpc7Mdwkdmm3Fu6l3RfEFT28Ij
fy97LMYv8W7beemDFqdmneb2w2ww0ZAFJg+GqIJZ9s/JadiFBDNU7CmJMhA225Qz
XC8ovejiePslnL4QJWlhVG93ZlBJ6SDkRgfcoIW2x4IBE6wv7jmRF4lOvb3z1ddP
iPQqhbEEbwMpXmWv7/2RnjAHdjdGaWRMC5+CaI+lqHyj6ir1c+e6u1QUY54qjmgM
koN/frqYab5Ek3kauj1iqW7rPkrFCqT2evh0YRqb1bFsCLJrRNxnOZ5wKXV/OYQa
QX5t0wFGCZ0KlbXDiw==
-----END CERTIFICATE-----
-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDJLdmTLefwmUsH
E8vOBSkpljG/NFp15YuZZVsr1vKjTxLRKzwlqjsDLXAe2ctufnavMJfohfWwB7VM
BPvnkaTsm0sySyG2PoXwE156UTITWCwETme1jm/ItOb+NlQgrPZn1Mueq9BOanqA
nrwEonU2hueweIBwpzqEjuSZj0KiyGYCLmqIpgGDO/+2GOLTT3e+1VJr2LAeFCn4
AQgyGS8zllmSJTl4ToxP1IERxVYqZYXbxSwHTh1pjBLYwonmXp3ifQtR0AbQUG3W
UKhP2vBQOP2XcyqaL3neDDbotSs2ticz/w3aOXsK7LDlqUhZpQOu3mL6m7zB1Z/N
WT587vHxAgMBAAECggEAJb0qIYftCJ9ZCbzW8JDbRefc8SdbCN7Er0PqNHEgFy6Q
MxjPMambZF8ztzXYCaRDk12kQYRPsHPhuJ7+ulQCAjinhIm/izZzXbPkd0GgCSzz
JOOoZNCRe68j3fBHG9IWbyfmAp/sdalXzaT5VE09e7sW323bekaEnbVIgN30/CAS
gI77YdaIhG+PT/pSCOc11MTkBJp+VhT1tEtlRAR78b1RXbGi1oUHRee7C3Ia8IKQ
3L5dPxR9RsYsR2O66908kEi8ZcuIjcbIuRPDXYHY+5Nwm3mXuZlkyjyfxJXsIA8i
qBrQrSpHGgAn1TVlLDSCKPLbkRzBRRvAW0zL/cDTuQKBgQDq/9Yxx9QivAuUxxdE
u0VO5CzzZYFWhDxAXS3/wYyo1YnoPtUz/lGCvMWp0k2aaa0+KTXv2fRCUGSujHW7
Jfo4kuMPkauAhoXx9QJAcjoK0nNbYEaqoJyMoRID+Qb9XHkj+lmBTmMVgALCT9DI
HekHj/M3b7CknbfWv1sOZ/vpQwKBgQDbKEuP/DWQa9DC5nn5phHD/LWZLG/cMR4X
TmwM/cbfRxM/6W0+/KLAodz4amGRzVlW6ax4k26BSE8Zt/SiyA1DQRTeFloduoqW
iWF4dMeItxw2am+xLREwtoN3FgsJHu2z/O/0aaBAOMLUXIPIyiE4L6OnEPifE/pb
AM8EbM5auwKBgGhdABIRjbtzSa1kEYhbprcXjIL3lE4I4f0vpIsNuNsOInW62dKC
Yk6uaRY3KHGn9uFBSgvf/qMost310R8xCYPwb9htN/4XQAspZTubvv0pY0O0aQ3D
0GJ/8dFD2f/Q/pekyfUsC8Lzm8YRzkXhSqkqG7iF6Kviw08iolyuf2ijAoGBANaA
pRzDvWWisUziKsa3zbGnGdNXVBEPniUvo8A/b7RAK84lWcEJov6qLs6RyPfdJrFT
u3S00LcHICzLCU1+QsTt4U/STtfEKjtXMailnFrq5lk4aiPfOXEVYq1fTOPbesrt
Katu6uOQ6tjRyEbx1/vXXPV7Peztr9/8daMeIAdbAoGBAOYRJ1CzMYQKjWF32Uas
7hhQxyH1QI4nV56Dryq7l/UWun2pfwNLZFqOHD3qm05aznzNKvk9aHAsOPFfUUXO
7sp0Ge5FLMSw1uMNnutcVcMz37KAY2fOoE2xoLM4DU/H2NqDjeGCsOsU1ReRS1vB
J+42JGwBdLV99ruYKVKOWPh4
-----END PRIVATE KEY-----
```

</details>

<details>
<summary>Basic Auth Examples</summary>

- Basic Auth secrets require values for the `username` and `password` keys in K8s.
- In your secret store of choice, these values should be comma-separated as a single value - `myusername,mypassword`.
- The driver will separate the two values and assign them to the `username` and `password` keys respectively.

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: vault-basic
spec:
  provider: vault
  parameters:
    roleName: "csi"
    vaultAddress: "http://vault.vault:8200"
    objects: |
      - secretPath: "secret/data/basic1"
        objectName: "basic/basic1"
        secretKey: "credentials"
      - secretPath: "secret/data/basic2"
        objectName: "basic/basic2"
        secretKey: "credentials"
      - secretPath: "secret/data/basic3"
        objectName: "basic/basic3"
        secretKey: "credentials"
  syncOptions:
    syncAll: true
    type: kubernetes.io/basic-auth
---
kind: ServiceAccount
apiVersion: v1
metadata:
  name: basic-sa
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: busybox-deployment-basic
  labels:
    app: busybox-basic
spec:
  replicas: 1
  selector:
    matchLabels:
      app: busybox-basic
  template:
    metadata:
      labels:
        app: busybox-basic
    spec:
      terminationGracePeriodSeconds: 0
      serviceAccountName: basic-sa
      containers:
        - image: k8s.gcr.io/e2e-test-images/busybox:1.29
          name: busybox-tls
          imagePullPolicy: IfNotPresent
          command:
            - "/bin/sleep"
            - "10000"
          env:
            - name: BASIC_USERNAME_1
              valueFrom:
                secretKeyRef:
                  name: basic-basic1
                  key: username
            - name: BASIC_PASSWORD_1
              valueFrom:
                secretKeyRef:
                  name: basic-basic1
                  key: password
            - name: BASIC_USERNAME_2
              valueFrom:
                secretKeyRef:
                  name: basic-basic2
                  key: username
            - name: BASIC_PASSWORD_2
              valueFrom:
                secretKeyRef:
                  name: basic-basic2
                  key: password
            - name: BASIC_USERNAME_3
              valueFrom:
                secretKeyRef:
                  name: basic-basic3
                  key: username
            - name: BASIC_PASSWORD_3
              valueFrom:
                secretKeyRef:
                  name: basic-basic3
                  key: password
          volumeMounts:
            - name: secrets-store-inline
              mountPath: "/mnt/secrets-store"
              readOnly: true
      volumes:
        - name: secrets-store-inline
          csi:
            driver: secrets-store.csi.k8s.io
            readOnly: true
            volumeAttributes:
              secretProviderClass: "vault-basic"

```

</details>

You may know already that you want to sync all your secrets to K8s using the CSI driver, but adding each secret to the SecretProviderClass configuration can be time-consuming and error-prone. In this case, you can tell the driver to sync all mounted secrets to K8s with the **SyncAll** option.

> NOTE: This feature is only available for secrets-store.csi.x-k8s.io/v1

See the full examples above to get a better understanding for the configuration shown:

```yaml
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
metadata:
  name: vault-opaque
spec:
  provider: vault
  parameters:
    roleName: "csi"
    vaultAddress: "http://vault.vault:8200"
    objects: |
      - secretPath: "secret/data/db-creds"
        objectName: "username"
        secretKey: "username"
      - secretPath: "secret/data/db-creds"
        objectName: "password"
        secretKey: "password"
  syncOptions:
    syncAll: true
    type: Opaque
```