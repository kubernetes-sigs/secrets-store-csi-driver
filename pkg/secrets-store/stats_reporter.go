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
	nodePublishTotal        metric.Int64Counter
	nodeUnPublishTotal      metric.Int64Counter
	nodePublishErrorTotal   metric.Int64Counter
	nodeUnPublishErrorTotal metric.Int64Counter
)

type reporter struct {
	meter metric.Meter
}

type StatsReporter interface {
	reportNodePublishMetrics(provider string)
	reportNodeUnPublishMetrics()
	reportNodePublishErrorMetrics(provider, errType string)
	reportNodeUnPublishErrorMetrics()
}

func newStatsReporter() StatsReporter {
	meter := global.Meter("secretsstore")
	nodePublishTotal = metric.Must(meter).NewInt64Counter("total_node_publish", metric.WithDescription("Total number of node publish calls"), metric.WithKeys(core.Key(providerKey)))
	nodeUnPublishTotal = metric.Must(meter).NewInt64Counter("total_node_unpublish", metric.WithDescription("Total number of node unpublish calls"))
	nodePublishErrorTotal = metric.Must(meter).NewInt64Counter("total_node_publish_error", metric.WithDescription("Total number of node publish calls with error"), metric.WithKeys(core.Key(providerKey), core.Key(errorKey)))
	nodeUnPublishErrorTotal = metric.Must(meter).NewInt64Counter("total_node_unpublish_error", metric.WithDescription("Total number of node unpublish calls with error"))
	return &reporter{meter: meter}
}

func (r *reporter) reportNodePublishMetrics(provider string) {
	labels := []core.KeyValue{key.String(providerKey, provider)}
	nodePublishTotal.Add(context.Background(), 1, labels...)
}

func (r *reporter) reportNodeUnPublishMetrics() {
	nodeUnPublishTotal.Add(context.Background(), 1, core.KeyValue{})
}

func (r *reporter) reportNodePublishErrorMetrics(provider, errType string) {
	labels := []core.KeyValue{key.String(providerKey, provider), key.String(errorKey, errType)}
	nodePublishErrorTotal.Add(context.Background(), 1, labels...)
}

func (r *reporter) reportNodeUnPublishErrorMetrics() {
	nodeUnPublishErrorTotal.Add(context.Background(), 1, core.KeyValue{})
}
