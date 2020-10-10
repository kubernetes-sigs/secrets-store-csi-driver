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

package pod

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WaitForPodInput struct {
	GetLister framework.GetLister
	Namespace string
	Labels    map[string]string
}

func WaitForPod(ctx context.Context, input WaitForPodInput) {
	e2e.Byf("%s: Waiting for pod(s) is running, labels: %q", input.Namespace, input.Labels)

	Eventually(func() error {
		pods := &corev1.PodList{}
		err := input.GetLister.List(ctx, pods, &client.ListOptions{
			Namespace:     input.Namespace,
			LabelSelector: labels.SelectorFromValidatedSet(labels.Set(input.Labels)),
		})
		if err != nil {
			return err
		}

		if len(pods.Items) == 0 {
			return errors.New("pod does not exist")
		}

	OUTER:
		for _, pod := range pods.Items {
			for _, cond := range pod.Status.Conditions {
				if cond.Type != corev1.PodReady {
					continue
				}
				if cond.Status == corev1.ConditionTrue {
					continue OUTER
				}
			}
			return fmt.Errorf("pod is not ready: %s", pod.Name)
		}

		return nil
	}).Should(Succeed())
}
