/*
Copyright 2020 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics

import (
	"flag"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/sdk/metric/controller/push"

	"k8s.io/klog/v2"
)

var (
	metricsBackend = flag.String("metrics-backend", "Prometheus", "Backend used for metrics")
	prometheusPort = flag.Int("prometheus-port", 8888, "Prometheus port for metrics backend")
)

const prometheusExporter = "prometheus"

func NewMetricsExporter() (m *push.Controller, err error) {
	mb := strings.ToLower(*metricsBackend)
	klog.Infof("metrics backend: %s", mb)
	switch mb {
	// Prometheus is the only exporter for now
	case prometheusExporter:
		m, err = newPrometheusExporter()
	default:
		err = fmt.Errorf("unsupported metrics backend %v", *metricsBackend)
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}
