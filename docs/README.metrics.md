# Metrics provided by Secrets Store CSI Driver

The Secrets Store CSI Driver uses [opentelemetry](https://opentelemetry.io/) for reporting metrics. This project is under [active development](https://github.com/open-telemetry/opentelemetry-go#release-schedule)

Prometheus is the only exporter that's currently supported with the driver.

## List of metrics provided by the driver

| Metric | Description | Tags |
| total_node_publish | Total number of successful volume mount requests | `provider=<provider name>` |
| total_node_unpublish | Total number of successful volume unmount requests | `""` |
| total_node_publish_error | Total number of errors with volume mount requests | `provider=<provider name>`<br>`error_type=<error code>` |
| total_node_unpublish_error | Total number of errors with volume unmount requests | `""` |
| total_sync_k8s_secret | Total number of k8s secrets synced | `namespace=<secret namespace>` |
| sync_k8s_secret_duration_sec | Distribution of how long it took to sync k8s secret | `""` |

**Sample Metrics output**

```shell
# HELP sync_k8s_secret_duration_sec Distribution of how long it took to sync k8s secret
# TYPE sync_k8s_secret_duration_sec histogram
sync_k8s_secret_duration_sec_bucket{le="0.1"} 0
sync_k8s_secret_duration_sec_bucket{le="0.2"} 0
sync_k8s_secret_duration_sec_bucket{le="0.3"} 0
sync_k8s_secret_duration_sec_bucket{le="0.4"} 1
sync_k8s_secret_duration_sec_bucket{le="0.5"} 1
sync_k8s_secret_duration_sec_bucket{le="1"} 1
sync_k8s_secret_duration_sec_bucket{le="1.5"} 1
sync_k8s_secret_duration_sec_bucket{le="2"} 1
sync_k8s_secret_duration_sec_bucket{le="2.5"} 1
sync_k8s_secret_duration_sec_bucket{le="3"} 1
sync_k8s_secret_duration_sec_bucket{le="5"} 1
sync_k8s_secret_duration_sec_bucket{le="10"} 1
sync_k8s_secret_duration_sec_bucket{le="15"} 1
sync_k8s_secret_duration_sec_bucket{le="30"} 1
sync_k8s_secret_duration_sec_bucket{le="+Inf"} 1
sync_k8s_secret_duration_sec_sum 0.3172117
sync_k8s_secret_duration_sec_count 1
# HELP total_node_publish Total number of node publish calls
# TYPE total_node_publish counter
total_node_publish{provider="azure"} 1
# HELP total_node_publish_error Total number of node publish calls with error
# TYPE total_node_publish_error counter
total_node_publish_error{error_type="ProviderBinaryNotFound",provider="azure-kv"} 4
total_node_publish_error{error_type="SecretProviderClassNotFound",provider=""} 1
# HELP total_sync_k8s_secret Total number of k8s secrets synced
# TYPE total_sync_k8s_secret counter
total_sync_k8s_secret{namespace="default"} 1
```
