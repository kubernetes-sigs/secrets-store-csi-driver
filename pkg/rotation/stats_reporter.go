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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	scope = "sigs.k8s.io/secrets-store-csi-driver"
)

var (
	providerKey = "provider"
	errorKey    = "error_type"
	osTypeKey   = "os_type"
	rotatedKey  = "rotated"
	runtimeOS   = runtime.GOOS
)

type reporter struct {
	rotationReconcileTotal      metric.Int64Counter
	rotationReconcileErrorTotal metric.Int64Counter
	rotationReconcileDuration   metric.Float64Histogram
}

type StatsReporter interface {
	reportRotationCtMetric(ctx context.Context, provider string, wasRotated bool)
	reportRotationErrorCtMetric(ctx context.Context, provider, errType string, wasRotated bool)
	reportRotationDuration(ctx context.Context, duration float64)
}

func newStatsReporter() (StatsReporter, error) {
	var err error

	r := &reporter{}
	meter := otel.Meter(scope)

	if r.rotationReconcileTotal, err = meter.Int64Counter("rotation_reconcile", metric.WithDescription("Total number of rotation reconciles")); err != nil {
		return nil, err
	}
	if r.rotationReconcileErrorTotal, err = meter.Int64Counter("rotation_reconcile_error", metric.WithDescription("Total number of rotation reconciles with error")); err != nil {
		return nil, err
	}
	if r.rotationReconcileDuration, err = meter.Float64Histogram("rotation_reconcile_duration_sec", metric.WithDescription("Distribution of how long it took to rotate secrets-store content for pods")); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *reporter) reportRotationCtMetric(ctx context.Context, provider string, wasRotated bool) {
	opt := metric.WithAttributes(
		attribute.Key(providerKey).String(provider),
		attribute.Key(osTypeKey).String(runtimeOS),
		attribute.Key(rotatedKey).Bool(wasRotated),
	)
	r.rotationReconcileTotal.Add(ctx, 1, opt)
}

func (r *reporter) reportRotationErrorCtMetric(ctx context.Context, provider, errType string, wasRotated bool) {
	opt := metric.WithAttributes(
		attribute.Key(providerKey).String(provider),
		attribute.Key(errorKey).String(errType),
		attribute.Key(osTypeKey).String(runtimeOS),
		attribute.Key(rotatedKey).Bool(wasRotated),
	)
	r.rotationReconcileErrorTotal.Add(ctx, 1, opt)
}

func (r *reporter) reportRotationDuration(ctx context.Context, duration float64) {
	opt := metric.WithAttributes(
		attribute.Key(osTypeKey).String(runtimeOS),
	)
	r.rotationReconcileDuration.Record(ctx, duration, opt)
}
