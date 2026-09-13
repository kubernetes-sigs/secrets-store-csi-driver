/*
Copyright 2026 The Kubernetes Authors.

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

package rotation

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestStatsReporter(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previousProvider := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previousProvider)
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	reporter, err := newStatsReporter()
	require.NoError(t, err)
	ctx := context.Background()
	reporter.reportRotationCtMetric(ctx, "test-provider", true)
	reporter.reportRotationErrorCtMetric(ctx, "test-provider", "test-error", false)
	reporter.reportRotationDuration(ctx, 0.5)

	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &data))
	require.Len(t, data.ScopeMetrics, 1)
	metrics := make(map[string]metricdata.Aggregation)
	for _, metric := range data.ScopeMetrics[0].Metrics {
		metrics[metric.Name] = metric.Data
	}
	for _, name := range []string{"rotation_reconcile", "rotation_reconcile_error"} {
		t.Run(name, func(t *testing.T) {
			sum, ok := metrics[name].(metricdata.Sum[int64])
			require.True(t, ok)
			require.True(t, sum.IsMonotonic)
			require.Len(t, sum.DataPoints, 1)
			require.Equal(t, int64(1), sum.DataPoints[0].Value)
			attributes := []attribute.KeyValue{
				attribute.String("provider", "test-provider"),
				attribute.String("os_type", runtime.GOOS),
				attribute.Bool("rotated", name == "rotation_reconcile"),
			}
			if name == "rotation_reconcile_error" {
				attributes = append(attributes, attribute.String("error_type", "test-error"))
			}
			require.ElementsMatch(t, attributes, sum.DataPoints[0].Attributes.ToSlice())
		})
	}
	duration, ok := metrics["rotation_reconcile_duration_sec"].(metricdata.Histogram[float64])
	require.True(t, ok)
	require.Len(t, duration.DataPoints, 1)
	require.Equal(t, uint64(1), duration.DataPoints[0].Count)
	require.Equal(t, 0.5, duration.DataPoints[0].Sum)
	require.ElementsMatch(t, []attribute.KeyValue{attribute.String("os_type", runtime.GOOS)}, duration.DataPoints[0].Attributes.ToSlice())
}
