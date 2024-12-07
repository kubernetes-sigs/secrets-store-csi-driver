# Metrics provided by Secrets Store CSI Driver

The Secrets Store CSI Driver uses [opentelemetry](https://opentelemetry.io/) for reporting metrics. This project is under [active development](https://github.com/open-telemetry/opentelemetry-go#release-schedule)

Prometheus is the only exporter that's currently supported with the driver.

## List of metrics provided by the driver

| Metric                          | Description                                                               | Tags                                                                                                                                                                                             |
|---------------------------------|---------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| node_publish_total              | Total number of successful volume mount requests                          | `os_type=<runtime os>`<br>`provider=<provider name>`<br>`pod_name=<pod_name>`<br>`pod_namespace=<pod_namespace>`<br>`secret_provider_class=<secret_provider_class>`                              |
| node_unpublish_total            | Total number of successful volume unmount requests                        | `os_type=<runtime os>`                                                                                                                                                                           |
| node_publish_error_total        | Total number of errors with volume mount requests                         | `os_type=<runtime os>`<br>`provider=<provider name>`<br>`error_type=<error code>`<br>`pod_name=<pod_name>`<br>`pod_namespace=<pod_namespace>`<br>`secret_provider_class=<secret_provider_class>` |
| node_unpublish_error_total      | Total number of errors with volume unmount requests                       | `os_type=<runtime os>`                                                                                                                                                                           |
| sync_k8s_secret_total           | Total number of k8s secrets synced                                        | `os_type=<runtime os>`<br>`provider=<provider name>`<br>`namespace=<namespace>`<br>`secret_provider_class=<secret_provider_class>`                                                               |
| sync_k8s_secret_duration_sec    | Distribution of how long it took to sync k8s secret                       | `os_type=<runtime os>`                                                                                                                                                                           |
| rotation_reconcile_total        | Total number of rotation reconciles                                       | `os_type=<runtime os>`<br>`rotated=<true or false>`<br>`pod_name=<pod_name>`<br>`pod_namespace=<pod_namespace>`<br>`secret_provider_class=<secret_provider_class>`                               |
| rotation_reconcile_error_total  | Total number of rotation reconciles with error                            | `os_type=<runtime os>`<br>`rotated=<true or false>`<br>`error_type=<error code>`<br>`pod_name=<pod_name>`<br>`pod_namespace=<pod_namespace>`<br>`secret_provider_class=<secret_provider_class>`  |
| rotation_reconcile_duration_sec | Distribution of how long it took to rotate secrets-store content for pods | `os_type=<runtime os>`<br>`pod_name=<pod_name>`<br>`pod_namespace=<pod_namespace>`<br>`secret_provider_class=<secret_provider_class>`                                                            |

Metrics are served from port 8095, but this port is not exposed outside the pod by default. Use kubectl port-forward to access the metrics over localhost:

```bash
kubectl port-forward ds/csi-secrets-store -n kube-system 8095:8095 &
curl localhost:8095/metrics
```

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

# HELP sync_k8s_secret_total Total number of k8s secrets synced
# TYPE sync_k8s_secret_total counter
sync_k8s_secret_total{namespace="csi-test-secret-ns",os_type="linux",provider="azure",secret_provider_class="csi-test-spc"} 1

# HELP rotation_reconcile_duration_sec Distribution of how long it took to rotate secrets-store content for pods
# TYPE rotation_reconcile_duration_sec histogram
rotation_reconcile_duration_sec_bucket{os_type="linux",le="0.1"} 0
rotation_reconcile_duration_sec_bucket{os_type="linux",le="0.2"} 0
rotation_reconcile_duration_sec_bucket{os_type="linux",le="0.3"} 0
rotation_reconcile_duration_sec_bucket{os_type="linux",le="0.4"} 1
rotation_reconcile_duration_sec_bucket{os_type="linux",le="0.5"} 1
rotation_reconcile_duration_sec_bucket{os_type="linux",le="1"} 1
rotation_reconcile_duration_sec_bucket{os_type="linux",le="1.5"} 1
rotation_reconcile_duration_sec_bucket{os_type="linux",le="2"} 1
rotation_reconcile_duration_sec_bucket{os_type="linux",le="2.5"} 1
rotation_reconcile_duration_sec_bucket{os_type="linux",le="3"} 1
rotation_reconcile_duration_sec_bucket{os_type="linux",le="5"} 1
rotation_reconcile_duration_sec_bucket{os_type="linux",le="10"} 1
rotation_reconcile_duration_sec_bucket{os_type="linux",le="15"} 1
rotation_reconcile_duration_sec_bucket{os_type="linux",le="30"} 1
rotation_reconcile_duration_sec_bucket{os_type="linux",le="+Inf"} 1
rotation_reconcile_duration_sec_sum{os_type="linux",} 0.3115892
rotation_reconcile_duration_sec_count{os_type="linux"} 1
# HELP rotation_reconcile_total Total number of rotation reconciles
# TYPE rotation_reconcile_total counter
rotation_reconcile_total{os_type="linux",pod_name="csi-test-app-wcsxk",pod_namespace="csi-test-secret-ns",provider="azure",rotated="false",secret_provider_class="csi-test-spc"} 1
# HELP rotation_reconcile_error_total Total number of rotation reconciles with error
# TYPE rotation_reconcile_error_total counter
rotation_reconcile_error_total{error_type="GRPCProviderError",os_type="linux",pod_name="csi-test-app-wcsxk",pod_namespace="csi-test-secret-ns",provider="azure",rotated="false",secret_provider_class="csi-test-spc"} 12
# HELP node_publish_total Total number of node publish calls
# TYPE node_publish_total counter
node_publish_total{os_type="linux",pod_name="csi-test-app-wcsxk",pod_namespace="csi-test-secret-ns",provider="azure",secret_provider_class="csi-test-spc"} 1
# HELP node_publish_error_total Total number of node publish calls with error
# TYPE node_publish_error_total counter
node_publish_error_total{error_type="ProviderBinaryNotFound",os_type="linux",pod_name="csi-test-app-wcsxk",pod_namespace="csi-test-secret-ns",provider="azure",secret_provider_class="csi-test-spc"} 7
# HELP node_unpublish_total Total number of node unpublish calls
# TYPE node_unpublish_total counter
node_unpublish_total{os_type="linux"} 1
```