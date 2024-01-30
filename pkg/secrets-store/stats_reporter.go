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
	"runtime"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
)

const (
	scope = "sigs.k8s.io/secrets-store-csi-driver"
)

var (
	providerKey     = "provider"
	errorKey        = "error_type"
	osTypeKey       = "os_type"
	runtimeOS       = runtime.GOOS
	podNameKey      = "pod_name"
	podNamespaceKey = "pod_namespace"
	spcKey          = "secret_provider_class"
)

type reporter struct {
	nodePublishTotal        metric.Int64Counter
	nodeUnPublishTotal      metric.Int64Counter
	nodePublishErrorTotal   metric.Int64Counter
	nodeUnPublishErrorTotal metric.Int64Counter
}

type StatsReporter interface {
	ReportNodePublishCtMetric(ctx context.Context, provider, podName, podNamespace, spc string)
	ReportNodeUnPublishCtMetric(ctx context.Context)
	ReportNodePublishErrorCtMetric(ctx context.Context, provider, podName, podNamespace, spc, errType string)
	ReportNodeUnPublishErrorCtMetric(ctx context.Context)
}

func NewStatsReporter() (StatsReporter, error) {
	var err error

	r := &reporter{}
	meter := global.Meter(scope)

	if r.nodePublishTotal, err = meter.Int64Counter("node_publish", metric.WithDescription("Total number of node publish calls")); err != nil {
		return nil, err
	}
	if r.nodeUnPublishTotal, err = meter.Int64Counter("node_unpublish", metric.WithDescription("Total number of node unpublish calls")); err != nil {
		return nil, err
	}
	if r.nodePublishErrorTotal, err = meter.Int64Counter("node_publish_error", metric.WithDescription("Total number of node publish calls with error")); err != nil {
		return nil, err
	}
	if r.nodeUnPublishErrorTotal, err = meter.Int64Counter("node_unpublish_error", metric.WithDescription("Total number of node unpublish calls with error")); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *reporter) ReportNodePublishCtMetric(ctx context.Context, provider, podName, podNamespace, spc string) {
	opt := metric.WithAttributes(
		attribute.Key(providerKey).String(provider),
		attribute.Key(osTypeKey).String(runtimeOS),
		attribute.Key(podNameKey).String(podName),
		attribute.Key(podNamespaceKey).String(podNamespace),
		attribute.Key(spcKey).String(spc),
	)
	r.nodePublishTotal.Add(ctx, 1, opt)
}

func (r *reporter) ReportNodeUnPublishCtMetric(ctx context.Context) {
	opt := metric.WithAttributes(
		attribute.Key(osTypeKey).String(runtimeOS),
	)
	r.nodeUnPublishTotal.Add(ctx, 1, opt)
}

func (r *reporter) ReportNodePublishErrorCtMetric(ctx context.Context, provider, podName, podNamespace, spc, errType string) {
	opt := metric.WithAttributes(
		attribute.Key(providerKey).String(provider),
		attribute.Key(errorKey).String(errType),
		attribute.Key(osTypeKey).String(runtimeOS),
		attribute.Key(podNameKey).String(podName),
		attribute.Key(podNamespaceKey).String(podNamespace),
		attribute.Key(spcKey).String(spc),
	)
	r.nodePublishErrorTotal.Add(ctx, 1, opt)
}

func (r *reporter) ReportNodeUnPublishErrorCtMetric(ctx context.Context) {
	opt := metric.WithAttributes(
		attribute.Key(osTypeKey).String(runtimeOS),
	)
	r.nodeUnPublishErrorTotal.Add(ctx, 1, opt)
}
