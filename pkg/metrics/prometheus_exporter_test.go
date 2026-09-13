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

package metrics

import (
	"context"
	"runtime"
	"testing"

	secretsstore "sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func TestPrometheusExporter(t *testing.T) {
	registry := prometheus.NewRegistry()
	previousRegistry := crmetrics.Registry
	previousProvider := otel.GetMeterProvider()
	crmetrics.Registry = registry
	t.Cleanup(func() {
		crmetrics.Registry = previousRegistry
		otel.SetMeterProvider(previousProvider)
	})

	require.NoError(t, initPrometheusExporter())
	provider, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	reporter, err := secretsstore.NewStatsReporter()
	require.NoError(t, err)
	ctx := context.Background()
	reporter.ReportNodePublishCtMetric(ctx, "test-provider")
	reporter.ReportSyncK8SecretDuration(ctx, 0.5)

	families, err := registry.Gather()
	require.NoError(t, err)
	metrics := make(map[string]*dto.MetricFamily)
	for _, family := range families {
		metrics[family.GetName()] = family
	}

	counter := metrics["node_publish_total"]
	require.NotNil(t, counter)
	require.Equal(t, dto.MetricType_COUNTER, counter.GetType())
	require.Len(t, counter.Metric, 1)
	require.Equal(t, float64(1), counter.Metric[0].GetCounter().GetValue())
	labels := make(map[string]string)
	for _, label := range counter.Metric[0].Label {
		labels[label.GetName()] = label.GetValue()
	}
	require.Equal(t, "test-provider", labels["provider"])
	require.Equal(t, runtime.GOOS, labels["os_type"])

	duration := metrics["k8s_secret_duration_sec"]
	require.NotNil(t, duration)
	require.Equal(t, dto.MetricType_HISTOGRAM, duration.GetType())
	require.Len(t, duration.Metric, 1)
	labels = make(map[string]string)
	for _, label := range duration.Metric[0].Label {
		labels[label.GetName()] = label.GetValue()
	}
	require.Equal(t, runtime.GOOS, labels["os_type"])
	histogram := duration.Metric[0].GetHistogram()
	require.Equal(t, uint64(1), histogram.GetSampleCount())
	require.Equal(t, 0.5, histogram.GetSampleSum())
	boundaries := prometheus.ExponentialBucketsRange(0.1, 2, 11)
	require.Len(t, histogram.Bucket, len(boundaries))
	for i, boundary := range boundaries {
		require.Equal(t, boundary, histogram.Bucket[i].GetUpperBound())
		var count uint64
		if boundary >= 0.5 {
			count = 1
		}
		require.Equal(t, count, histogram.Bucket[i].GetCumulativeCount())
	}
}
