Secrets Store Sync Controller for Kubernetes secrets - Synchronizes secrets as Kubernetes secrets using the same interface as the [Secrets Store CSI Driver](https://github.com/kubernetes-sigs/secrets-store-csi-driver) and is a Kubernetes sub-project [Kubernetes SIG Auth sub-project](https://github.com/kubernetes/community/tree/master/sig-auth).

## Getting Started
1. The controller can be deployed as a deployment.  
1. The provider must be deployed as a container in the same pod as the controller. 
1. The controller watches for changes to the SecretStoreSync and SecretProviderClass objects and synchronizes the secrets defined in the SecretStoreSync as Kubernetes secrets.
1. The arguments passed to the controller are: 
    1. provider-volume - The volume name used to communicate with the provider. 
    1. token-request-audience - Token requests audience for the controller. This is similar to the configuration required for the Secret Store CSI Driver. Refer to [doc](https://datatracker.ietf.org/doc/html/rfc7519#section-4.1.3) for more information.
    1. rotation-poll-interval-in-seconds - The interval in seconds to poll for secret rotation.

