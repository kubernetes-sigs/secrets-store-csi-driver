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

package mocks // import sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store/mocks

import "context"

type MetricDetails struct {
	Provider     string
	PodName      string
	PodNamespace string
	Spc          string
	ErrorType    string
}

type FakeReporter struct {
	reportNodePublishCtMetricInvoked        int
	reportNodeUnPublishCtMetricInvoked      int
	reportNodePublishErrorCtMetricInvoked   int
	reportNodeUnPublishErrorCtMetricInvoked int
	metricDetails                           []MetricDetails
}

func NewFakeReporter() *FakeReporter {
	return &FakeReporter{
		metricDetails: []MetricDetails{},
	}
}

func (f *FakeReporter) ReportNodePublishCtMetric(ctx context.Context, provider, podName, podNamespace, spc string) {
	f.reportNodePublishCtMetricInvoked++
	f.metricDetails = append(f.metricDetails, MetricDetails{
		Provider:     provider,
		PodName:      podName,
		PodNamespace: podNamespace,
		Spc:          spc,
	})
}

func (f *FakeReporter) ReportNodeUnPublishCtMetric(ctx context.Context) {
	f.reportNodeUnPublishCtMetricInvoked++
}

func (f *FakeReporter) ReportNodePublishErrorCtMetric(ctx context.Context, provider, podName, podNamespace, spc, errType string) {
	f.reportNodePublishErrorCtMetricInvoked++
	f.metricDetails = append(f.metricDetails, MetricDetails{
		Provider:     provider,
		PodName:      podName,
		PodNamespace: podNamespace,
		Spc:          spc,
		ErrorType:    errType,
	})
}

func (f *FakeReporter) ReportNodeUnPublishErrorCtMetric(ctx context.Context) {
	f.reportNodeUnPublishErrorCtMetricInvoked++
}

func (f *FakeReporter) ReportNodePublishCtMetricInvoked() int {
	return f.reportNodePublishCtMetricInvoked
}
func (f *FakeReporter) ReportNodeUnPublishCtMetricInvoked() int {
	return f.reportNodeUnPublishCtMetricInvoked
}
func (f *FakeReporter) ReportNodePublishErrorCtMetricInvoked() int {
	return f.reportNodePublishErrorCtMetricInvoked
}
func (f *FakeReporter) ReportNodeUnPublishErrorCtMetricInvoked() int {
	return f.reportNodeUnPublishErrorCtMetricInvoked
}

func (f *FakeReporter) GetMetricDetails() []MetricDetails {
	return f.metricDetails
}
