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
	crprometheus "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func initPrometheusExporter() error {
	exporter, err := prometheus.New(
		prometheus.WithRegisterer(metrics.Registry.(*crprometheus.Registry)), // using the controller-runtime prometheus metrics registry
	)
	if err != nil {
		return err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithView(metric.NewView(
			metric.Instrument{Name: "secretsstore_*"},
			metric.Stream{
				Aggregation: aggregation.ExplicitBucketHistogram{
					Boundaries: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 1, 1.5, 2, 2.5, 3.0, 5.0, 10.0, 15.0, 30.0},
				}},
		)),
	)

	global.SetMeterProvider(meterProvider)

	return nil
}
