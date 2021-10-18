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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// SecretLister is a store used to list secrets
type SecretLister struct {
	cache.Store
}

// GetWithKey returns secret with key from the informer cache
func (sl *SecretLister) GetWithKey(key string) (*corev1.Secret, error) {
	sec, exists, err := sl.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: corev1.GroupName, Resource: "secrets"}, key)
	}
	secret, ok := sec.(*corev1.Secret)
	if !ok {
		return nil, errors.Errorf("failed to cast %T to secret", sec)
	}
	return secret, nil
}
