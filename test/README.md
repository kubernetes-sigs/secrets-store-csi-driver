# Testing

This doc lists the different Secrets Store CSI Driver scenarios tested as part of CI.

## E2E tests

| Test Category                                                                                                                                                                                                                                         | Azure | Vault | GCP |
| ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----- | ----- | --- |
| Mount tests<ul><li>CSI Inline volume test with Pod Portability</li></ul>                                                                                                                                                                              | ✔️     | ✔️     | ✔️   |
| Sync as Kubernetes secrets<ul><li>Check Kubernetes secret</li><li>Check owner references in secret with multiple owners</li><li>Check owner references updated when a owner is deleted</li><li>Check secret deleted when all owners deleted</li></ul> | ✔️     | ✔️     |     |
| Namespaced Scope SecretProviderClass<ul><li>Check `SecretProviderClass` in same namespace as pod</li></ul>                                                                                                                                            | ✔️     | ✔️     |     |
| Namespaced Scope SecretProviderClass negative test<ul><li>Check volume mount fails when `SecretProviderClass` not found in same namespace as pod</li></ul>                                                                                            | ✔️     | ✔️     |     |
| Multiple SecretProviderClass<ul><li>Check multiple CSI Inline volumes with different SecretProviderClass</li></ul>                                                                                                                                    | ✔️     | ✔️     |     |
| Autorotation of mount contents and Kubernetes secrets<ul><li>Check mount content and Kubernetes secret updated after rotation</li></ul>                                                                                                               | ✔️     |       |     |
| Test filtered watch for `nodePublishSecretRef` feature<ul><li>Check labelled nodePublishSecretRef accessible after upgrade to enable `filteredWatchSecret` feature</li></ul>                                                                          | ✔️     |       | ✔️   |
| Windows tests                                                                                                                                                                                                                                         | ✔️     |       |     |

## Sanity tests

CSI Driver sanity tests using [CSI Driver Sanity Tester](https://github.com/kubernetes-csi/csi-test/tree/master/pkg/sanity#csi-driver-sanity-tester) to ensure Secrets Store CSI Driver conforms to the CSI specification.
