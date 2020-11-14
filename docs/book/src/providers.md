# Providers

This project features a pluggable provider interface developers can implement that defines the actions of the Secrets Store CSI driver. This enables retrieval of sensitive objects stored in an enterprise-grade external secrets store into Kubernetes while continue to manage these objects outside of Kubernetes.

## Criteria for Supported Providers

Here is a list of criteria for supported provider:
1. Code audit of the provider implementation to ensure it adheres to the required provider-driver interface - [Implementing a Provider for Secrets Store CSI Driver](docs/README.new-provider.md)
2. Add provider to the e2e test suite to demonstrate it functions as expected https://github.com/kubernetes-sigs/secrets-store-csi-driver/tree/master/test/bats Please use existing providers e2e tests as a reference.
3. If any update is made by a provider (not limited to security updates), the provider is expected to update the provider's e2e test in this repo

## Removal from Supported Providers

Failure to adhere to the [Criteria for Supported Providers](#criteria-for-supported-providers) will result in the removal of the provider from the supported list and subject to another review before it can be added back to the list of supported providers.

When a provider's e2e tests are consistently failing with the latest version of the driver, the driver maintainers will coordinate with the provider maintainers to provide a fix. If the test failures are not resolved within 4 weeks, then the provider will be removed from the list of supported providers. 