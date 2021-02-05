# Metrics provided by Secrets Store CSI Driver

The Secrets Store CSI Driver uses [opentelemetry](https://opentelemetry.io/) for reporting metrics. This project is under [active development](https://github.com/open-telemetry/opentelemetry-go#release-schedule)

Prometheus is the only exporter that's currently supported with the driver.

## List of metrics provided by the driver

| Metric                          | Description                                                               | Tags                                                                              |
| ------------------------------- | ------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| total_node_publish              | Total number of successful volume mount requests                          | `os_type=<runtime os>`<br>`provider=<provider name>`                              |
| total_node_unpublish            | Total number of successful volume unmount requests                        | `os_type=<runtime os>`                                                            |
| total_node_publish_error        | Total number of errors with volume mount requests                         | `os_type=<runtime os>`<br>`provider=<provider name>`<br>`error_type=<error code>` |
| total_node_unpublish_error      | Total number of errors with volume unmount requests                       | `os_type=<runtime os>`                                                            |
| total_sync_k8s_secret           | Total number of k8s secrets synced                                        | `os_type=<runtime os>`<br>`provider=<provider name>`                              |
| sync_k8s_secret_duration_sec    | Distribution of how long it took to sync k8s secret                       | `os_type=<runtime os>`                                                            |
| total_rotation_reconcile        | Total number of rotation reconciles                                       | `os_type=<runtime os>`<br>`rotated=<true or false>`                               |
| total_rotation_reconcile_error  | Total number of rotation reconciles with error                            | `os_type=<runtime os>`<br>`rotated=<true or false>`<br>`error_type=<error code>`  |
| rotation_reconcile_duration_sec | Distribution of how long it took to rotate secrets-store content for pods | `os_type=<runtime os>`                                                            |

### Sample Metrics output

```shell
# HELP sync_k8s_secret_duration_sec Distribution of how long it took to sync k8s secret
# TYPE sync_k8s_secret_duration_sec histogram
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="0.1"} 0
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="0.2"} 0
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="0.3"} 0
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="0.4"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="0.5"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="1"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="1.5"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="2"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="2.5"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="3"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="5"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="10"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="15"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="30"} 1
sync_k8s_secret_duration_sec_bucket{os_type="linux",le="+Inf"} 1
sync_k8s_secret_duration_sec_sum{os_type="linux"} 0.3115892
sync_k8s_secret_duration_sec_count{os_type="linux"} 1
# HELP total_node_publish Total number of node publish calls
# TYPE total_node_publish counter
total_node_publish{os_type="linux",provider="azure"} 1
# HELP total_node_publish_error Total number of node publish calls with error
# TYPE total_node_publish_error counter
total_node_publish_error{error_type="ProviderBinaryNotFound",os_type="linux",provider="azure"} 2
total_node_publish_error{error_type="SecretProviderClassNotFound",os_type="linux",provider=""} 4
# HELP total_node_unpublish Total number of node unpublish calls
# TYPE total_node_unpublish counter
total_node_unpublish{os_type="linux"} 1
# HELP total_sync_k8s_secret Total number of k8s secrets synced
# TYPE total_sync_k8s_secret counter
total_sync_k8s_secret{os_type="linux",provider="azure"} 1
```
