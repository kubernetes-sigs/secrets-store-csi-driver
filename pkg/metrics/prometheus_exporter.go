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
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/exporters/metric/prometheus"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
)

func newPrometheusExporter() (*push.Controller, error) {
	/*
		Prometheus exporter for opentelemetry is under active development
		Histogram support was added in v0.4.3 - https://github.com/open-telemetry/opentelemetry-go/pull/601
		Defining the buckets is due to change in future release - https://github.com/open-telemetry/opentelemetry-go/issues/689
	*/
	pusher, hf, err := prometheus.InstallNewPipeline(prometheus.Config{
		DefaultHistogramBoundaries: []core.Number{
			core.NewFloat64Number(0.1),
			core.NewFloat64Number(0.2),
			core.NewFloat64Number(0.3),
			core.NewFloat64Number(0.4),
			core.NewFloat64Number(0.5),
			core.NewFloat64Number(1),
			core.NewFloat64Number(1.5),
			core.NewFloat64Number(2),
			core.NewFloat64Number(2.5),
			core.NewFloat64Number(3.0),
			core.NewFloat64Number(5.0),
			core.NewFloat64Number(10.0),
			core.NewFloat64Number(15.0),
			core.NewFloat64Number(30.0),
		}})
	if err != nil {
		return nil, err
	}
	http.HandleFunc("/", hf)
	go func() {
		_ = http.ListenAndServe(fmt.Sprintf(":%v", *prometheusPort), nil)
	}()

	return pusher, nil
}
