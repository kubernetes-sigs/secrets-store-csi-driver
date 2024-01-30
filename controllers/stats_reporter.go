/*
Copyright 2024 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"runtime"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
)

const (
	scope = "sigs.k8s.io/secrets-store-csi-driver"
)

var (
	providerKey  = "provider"
	osTypeKey    = "os_type"
	runtimeOS    = runtime.GOOS
	namespaceKey = "namespace"
	spcKey       = "secret_provider_class"
)

type reporter struct {
	syncK8sSecretTotal    metric.Int64Counter
	syncK8sSecretDuration metric.Float64Histogram
}

type StatsReporter interface {
	ReportSyncSecretCtMetric(ctx context.Context, provider, namespace, spc string)
	ReportSyncSecretDuration(ctx context.Context, duration float64)
}

func newStatsReporter() (StatsReporter, error) {
	var err error

	r := &reporter{}
	meter := global.Meter(scope)

	if r.syncK8sSecretTotal, err = meter.Int64Counter("sync_k8s_secret", metric.WithDescription("Total number of k8s secrets synced")); err != nil {
		return nil, err
	}
	if r.syncK8sSecretDuration, err = meter.Float64Histogram("sync_k8s_secret_duration_sec", metric.WithDescription("Distribution of how long it took to sync k8s secret")); err != nil {
		return nil, err
	}
	return r, nil
}

func (r reporter) ReportSyncSecretCtMetric(ctx context.Context, provider, namespace, spc string) {
	opt := metric.WithAttributes(
		attribute.Key(providerKey).String(provider),
		attribute.Key(osTypeKey).String(runtimeOS),
		attribute.Key(namespaceKey).String(namespace),
		attribute.Key(spcKey).String(spc),
	)
	r.syncK8sSecretTotal.Add(ctx, 1, opt)
}

func (r reporter) ReportSyncSecretDuration(ctx context.Context, duration float64) {
	opt := metric.WithAttributes(
		attribute.Key(osTypeKey).String(runtimeOS),
	)
	r.syncK8sSecretDuration.Record(ctx, duration, opt)
}
