# Command Reference

The Secrets Store CSI Driver can be provided with the following command line arguments.

The `secrets-store` container in the DaemonSet can be configured using the following command line arguments:

## List of command line options

| Parameter                        | Description                                                            | Default                                     |
|----------------------------------|------------------------------------------------------------------------|---------------------------------------------|
| `--endpoint`                         | CSI endpoint                                                           | `unix://tmp/csi.sock`                         |
| `--drivername`                       | Name of the driver                                                     | `secrets-store.csi.k8s.io`                    |
| `--nodeid`                           | Node ID                                                                |                                             |
| `--log-format-json`                  | Set log formatter to json                                              | `false`                                       |
| `--provider-volume`                  | Volume path for provider                                               | `/etc/kubernetes/secrets-store-csi-providers` |
| `--additional-provider-volume-paths` | Comma separated list of additional paths to communicate with providers | `/var/run/secrets-store-csi-providers`        |
| `--metrics-addr`                     | The address the metric endpoint binds to                               | `:8095`                                       |
| `--enable-pprof`                     | Enable pprof profiling                                                 | `false`                                       |
| `--pprof-port`                       | Port for pprof profiling                                               | `6065`                                        |
| `--max-call-recv-msg-size`           | Maximum size in bytes of gRPC response from plugins                    | `4194304`                                     |
| `--provider-health-check`            	| Enable health check for configured providers                           	| `false`                                       	|
| `--provider-health-check-interval`   	| Provider healthcheck interval duration                                 	|  `2m`                                           	|