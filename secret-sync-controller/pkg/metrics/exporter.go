package metrics

import (
	"flag"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

var (
	metricsBackend = flag.String("metrics-backend", "Prometheus", "Backend used for metrics")
)

const prometheusExporter = "prometheus"

func InitMetricsExporter() error {
	mb := strings.ToLower(*metricsBackend)
	klog.InfoS("initializing metrics backend", "backend", mb)
	switch mb {
	// Prometheus is the only supported exporter
	case prometheusExporter:
		return initPrometheusExporter()
	default:
		return fmt.Errorf("unsupported metrics backend %v", *metricsBackend)
	}
}
