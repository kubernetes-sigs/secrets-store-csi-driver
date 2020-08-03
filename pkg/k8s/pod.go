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

package k8s

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

// PodLister is a store used to list pods
type PodLister struct {
	cache.Store
}

// GetWithKey returns pod with key from the informer cache
func (pl *PodLister) GetWithKey(key string) (*v1.Pod, error) {
	p, exists, err := pl.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("pod not found in informer cache")
	}
	pod, ok := p.(*v1.Pod)
	if !ok {
		return nil, fmt.Errorf("failed to cast %T to %s", p, "pod")
	}
	return pod, nil
}
