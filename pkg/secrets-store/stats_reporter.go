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

package secretsstore

import (
	"context"

	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/key"
	"go.opentelemetry.io/otel/api/metric"
)

var (
	providerKey             = "provider"
	errorKey                = "error_type"
	namespaceKey            = "namespace"
	nodePublishTotal        metric.Int64Counter
	nodeUnPublishTotal      metric.Int64Counter
	nodePublishErrorTotal   metric.Int64Counter
	nodeUnPublishErrorTotal metric.Int64Counter
	syncK8sSecretTotal      metric.Int64Counter
	syncK8sSecretDuration   metric.Float64Measure
)

type reporter struct {
	meter metric.Meter
}

type StatsReporter interface {
	reportNodePublishCtMetric(provider string)
	reportNodeUnPublishCtMetric()
	reportNodePublishErrorCtMetric(provider, errType string)
	reportNodeUnPublishErrorCtMetric()
	reportSyncK8SecretCtMetric(namespace string, count int)
	reportSyncK8SecretDuration(duration float64)
}

func newStatsReporter() StatsReporter {
	meter := global.Meter("secretsstore")
	nodePublishTotal = metric.Must(meter).NewInt64Counter("total_node_publish", metric.WithDescription("Total number of node publish calls"))
	nodeUnPublishTotal = metric.Must(meter).NewInt64Counter("total_node_unpublish", metric.WithDescription("Total number of node unpublish calls"))
	nodePublishErrorTotal = metric.Must(meter).NewInt64Counter("total_node_publish_error", metric.WithDescription("Total number of node publish calls with error"))
	nodeUnPublishErrorTotal = metric.Must(meter).NewInt64Counter("total_node_unpublish_error", metric.WithDescription("Total number of node unpublish calls with error"))
	syncK8sSecretTotal = metric.Must(meter).NewInt64Counter("total_sync_k8s_secret", metric.WithDescription("Total number of k8s secrets synced"))
	syncK8sSecretDuration = metric.Must(meter).NewFloat64Measure("sync_k8s_secret_duration_sec", metric.WithDescription("Distribution of how long it took to sync k8s secret"))
	return &reporter{meter: meter}
}

func (r *reporter) reportNodePublishCtMetric(provider string) {
	labels := []core.KeyValue{key.String(providerKey, provider)}
	nodePublishTotal.Add(context.Background(), 1, labels...)
}

func (r *reporter) reportNodeUnPublishCtMetric() {
	nodeUnPublishTotal.Add(context.Background(), 1, []core.KeyValue{}...)
}

func (r *reporter) reportNodePublishErrorCtMetric(provider, errType string) {
	labels := []core.KeyValue{key.String(providerKey, provider), key.String(errorKey, errType)}
	nodePublishErrorTotal.Add(context.Background(), 1, labels...)
}

func (r *reporter) reportNodeUnPublishErrorCtMetric() {
	nodeUnPublishErrorTotal.Add(context.Background(), 1, []core.KeyValue{}...)
}

func (r *reporter) reportSyncK8SecretCtMetric(namespace string, count int) {
	labels := []core.KeyValue{key.String(namespaceKey, namespace)}
	syncK8sSecretTotal.Add(context.Background(), int64(count), labels...)
}

func (r *reporter) reportSyncK8SecretDuration(duration float64) {
	r.meter.RecordBatch(context.Background(), []core.KeyValue{}, syncK8sSecretDuration.Measurement(duration))
}
