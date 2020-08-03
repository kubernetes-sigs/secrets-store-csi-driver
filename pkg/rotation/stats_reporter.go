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

package rotation

import (
	"context"
	"runtime"

	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/key"
	"go.opentelemetry.io/otel/api/metric"
)

var (
	providerKey                 = "provider"
	errorKey                    = "error_type"
	osTypeKey                   = "os_type"
	rotatedKey                  = "rotated"
	rotationReconcileTotal      metric.Int64Counter
	rotationReconcileErrorTotal metric.Int64Counter
	rotationReconcileDuration   metric.Float64Measure
	runtimeOS                   = runtime.GOOS
)

type reporter struct {
	meter metric.Meter
}

type StatsReporter interface {
	reportRotationCtMetric(provider string, wasRotated bool)
	reportRotationErrorCtMetric(provider, errType string, wasRotated bool)
	reportRotationDuration(duration float64)
}

func newStatsReporter() StatsReporter {
	meter := global.Meter("secretsstore")
	rotationReconcileTotal = metric.Must(meter).NewInt64Counter("total_rotation_reconcile", metric.WithDescription("Total number of rotation reconciles"))
	rotationReconcileErrorTotal = metric.Must(meter).NewInt64Counter("total_rotation_reconcile_error", metric.WithDescription("Total number of rotation reconciles with error"))
	rotationReconcileDuration = metric.Must(meter).NewFloat64Measure("rotation_reconcile_duration_sec", metric.WithDescription("Distribution of how long it took to rotate secrets-store content for pods"))
	return &reporter{meter: meter}
}

func (r *reporter) reportRotationCtMetric(provider string, wasRotated bool) {
	labels := []core.KeyValue{key.String(providerKey, provider), key.String(osTypeKey, runtimeOS), key.Bool(rotatedKey, wasRotated)}
	rotationReconcileTotal.Add(context.Background(), 1, labels...)
}

func (r *reporter) reportRotationErrorCtMetric(provider, errType string, wasRotated bool) {
	labels := []core.KeyValue{key.String(providerKey, provider), key.String(errorKey, errType), key.String(osTypeKey, runtimeOS), key.Bool(rotatedKey, wasRotated)}
	rotationReconcileErrorTotal.Add(context.Background(), 1, labels...)
}

func (r *reporter) reportRotationDuration(duration float64) {
	r.meter.RecordBatch(context.Background(), []core.KeyValue{key.String(osTypeKey, runtimeOS)}, rotationReconcileDuration.Measurement(duration))
}
