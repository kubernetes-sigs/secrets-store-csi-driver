# Using Secrets Store CSI to Enable NGINX Ingress Controller with TLS

The Secrets Store CSI Driver can be used to enable applications to work with NGINX Ingress Controller with TLS stored in an External Secrets Store. 
For more information on securing an Ingress with TLS, refer to: https://kubernetes.io/docs/concepts/services-networking/ingress/#tls

Checkout provider samples on how to get started -

- [Using Secrets Store CSI and Azure Key Vault Provider](https://github.com/Azure/secrets-store-csi-driver-provider-azure/blob/master/docs/ingress-tls.md)
- [Using Secrets Store CSI and Hashicorp Vault Provider](https://github.com/hashicorp/secrets-store-csi-driver-provider-vault/blob/master/sample/ingress-controller-tls/README.md)