# Using Secrets Store CSI to Enable NGINX Ingress Controller with TLS
This guide demonstrates steps required to setup Secrets store csi driver to enable applications to work with NGINX Ingress Controller with TLS stored in an exteranl Secrets store. 
For more information on securing an Ingress with TLS, refer to: https://kubernetes.io/docs/concepts/services-networking/ingress/#tls

# Generate a TLS Cert

```bash
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -out ingress-tls.crt \
    -keyout ingress-tls.key \
    -subj "/CN=demo.test.com/O=ingress-tls"
```

# Store Cert in External Secrets Store Service
e.g. Azure Key Vault or Vault

# Deploy Secrets-store CSI and the Provider
https://github.com/kubernetes-sigs/secrets-store-csi-driver#usage

# Deploy Ingress Controller

Create a namespace

```bash
kubectl create ns ingress-test
```

Helm install ingress-controller

```bash
helm install stable/nginx-ingress --generate-name \                               
    --namespace ingress-test \
    --set controller.replicaCount=2 \
    --set controller.nodeSelector."beta\.kubernetes\.io/os"=linux \
    --set defaultBackend.nodeSelector."beta\.kubernetes\.io/os"=linux
```

# Deploy a SecretsProviderClass Resource
> NOTE: For this sample, we are using the `azure` provider. For more information, head over to: https://github.com/Azure/secrets-store-csi-driver-provider-azure#install-the-azure-key-vault-provider

```bash
kubectl apply -f sample/ingress-controller-tls/secretproviderclass-azure-tls.yaml -n ingress-test
```

# [OPTIONAL] Create a Secret Required by Provider

```bash
kubectl create secret generic secrets-store-creds --from-literal clientid=xxxx --from-literal clientsecret=xxxx -n ingress-test 
```

# Deploy Test Apps with Reference to Secrets Store CSI

> NOTE: These apps referece a secrets store csi volume and a `secretProviderClass` object created earlier. A Kubernetes secret `ingress-tls-csi` will be created by the CSI driver as a result of the app creation.

```yaml
      volumes:
        - name: secrets-store-inline
          csi:
            driver: secrets-store.csi.k8s.io
            readOnly: true
            volumeAttributes:
              secretProviderClass: "azure-tls"
            nodePublishSecretRef:
              name: secrets-store-creds
```

```bash
kubectl apply -f sample/ingress-controller-tls/deployment-app-one.yaml -n ingress-test
kubectl apply -f sample/ingress-controller-tls/deployment-app-two.yaml -n ingress-test

```

# Check for the Kubernetes Secret created by the CSI driver
```bash
kubectl get secret -n ingress-test

NAME                                             TYPE                                  DATA   AGE
ingress-tls-csi                                  kubernetes.io/tls                     2      1m34s
```

# Deploy an Ingress Resource Referencing the Secret created by the CSI driver

> NOTE: The ingress resource references the Kubernetes secret `ingress-tls-csi` created by the CSI driver as a result of the app creation.

```yaml
tls:
  - hosts:
    - demo.test.com
    secretName: ingress-tls-csi
```

```bash
kubectl apply -f sample/ingress-controller-tls/ingress.yaml -n ingress-test
```

# Get the External IP of the Ingress Controller

```bash
 kubectl get service -l app=nginx-ingress --namespace ingress-test                 ⎈ ritak8s116
NAME                                       TYPE           CLUSTER-IP     EXTERNAL-IP      PORT(S)                      AGE
nginx-ingress-1588032400-controller        LoadBalancer   10.0.255.157   52.xx.xx.xx   80:31293/TCP,443:31265/TCP   19m
nginx-ingress-1588032400-default-backend   ClusterIP      10.0.223.214   <none>           80/TCP                       19m
```

# Test Ingress with TLS
Using `curl` to verify ingress configuration using TLS. 
Replace the public IP with the external IP of the ingress controller service from the previous step.  

```bash
curl -v -k --resolve demo.test.com:443:52.xx.xx.xx https://demo.test.com

# You should see the following in your outpout
*  subject: CN=demo.test.com; O=ingress-tls
*  start date: Apr 15 04:23:46 2020 GMT
*  expire date: Apr 15 04:23:46 2021 GMT
*  issuer: CN=demo.test.com; O=ingress-tls
*  SSL certificate verify result: self signed certificate (18), continuing anyway.
```
