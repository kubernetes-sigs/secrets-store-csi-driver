/*
Copyright 2022 The Kubernetes Authors.

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
	"encoding/json"
	"fmt"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeinformers "k8s.io/client-go/informers"
	storageinformers "k8s.io/client-go/informers/storage/v1"
	"k8s.io/client-go/kubernetes"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/kubelet/token"
)

// TokenClient is a client for Kubernetes Token API
type TokenClient struct {
	driverName        string
	csiDriverInformer storageinformers.CSIDriverInformer
	csiDriverLister   storagelisters.CSIDriverLister
	manager           *token.Manager
}

// NewTokenClient creates a new TokenClient
// The client will be used to request a token for token requests configured in the CSIDriver.
func NewTokenClient(kubeClient kubernetes.Interface, driverName string, resyncPeriod time.Duration) *TokenClient {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resyncPeriod,
		kubeinformers.WithTweakListOptions(
			func(options *metav1.ListOptions) {
				options.FieldSelector = fmt.Sprintf("metadata.name=%s", driverName)
			},
		),
	)

	csiDriverInformer := kubeInformerFactory.Storage().V1().CSIDrivers()
	csiDriverLister := csiDriverInformer.Lister()

	return &TokenClient{
		driverName:        driverName,
		csiDriverInformer: csiDriverInformer,
		csiDriverLister:   csiDriverLister,
		manager:           token.NewManager(kubeClient),
	}
}

// Run initiates the sync of the informers and caches
func (c *TokenClient) Run(stopCh <-chan struct{}) error {
	go c.csiDriverInformer.Informer().Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.csiDriverInformer.Informer().HasSynced) {
		return fmt.Errorf("failed to sync informer caches")
	}
	return nil
}

// PodServiceAccountTokenAttrs returns the token for the pod service account that can be bound to the pod.
// This token will be sent to the providers and is of the format:
//
//	"csi.storage.k8s.io/serviceAccount.tokens": {
//	  <audience>: {
//	    'token': <token>,
//	    'expirationTimestamp': <expiration timestamp in RFC3339 format>,
//	  },
//	  ...
//	}
//
// ref: https://kubernetes-csi.github.io/docs/token-requests.html#usage
func (c *TokenClient) PodServiceAccountTokenAttrs(namespace, podName, serviceAccountName string, podUID types.UID) (map[string]string, error) {
	csiDriver, err := c.csiDriverLister.Get(c.driverName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(5).InfoS("CSIDriver not found, not adding service account token information", "driver", c.driverName)
			return nil, nil
		}
		return nil, err
	}

	if len(csiDriver.Spec.TokenRequests) == 0 {
		return nil, nil
	}

	outputs := map[string]authenticationv1.TokenRequestStatus{}
	for _, tokenRequest := range csiDriver.Spec.TokenRequests {
		audience := tokenRequest.Audience
		audiences := []string{audience}
		if audience == "" {
			audiences = []string{}
		}
		tr := &authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				ExpirationSeconds: tokenRequest.ExpirationSeconds,
				Audiences:         audiences,
				BoundObjectRef: &authenticationv1.BoundObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       podName,
					UID:        podUID,
				},
			},
		}

		tr, err := c.GetServiceAccountToken(namespace, serviceAccountName, tr)
		if err != nil {
			return nil, err
		}
		outputs[audience] = tr.Status
	}

	klog.V(4).InfoS("Fetched service account token attrs for CSIDriver", "driver", c.driverName, "podUID", podUID)
	tokens, err := json.Marshal(outputs)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"csi.storage.k8s.io/serviceAccount.tokens": string(tokens),
	}, nil
}

// GetServiceAccountToken gets a service account token for a pod from cache or
// from the TokenRequest API. This process is as follows:
// * Check the cache for the current token request.
// * If the token exists and does not require a refresh, return the current token.
// * Attempt to refresh the token.
// * If the token is refreshed successfully, save it in the cache and return the token.
// * If refresh fails and the old token is still valid, log an error and return the old token.
// * If refresh fails and the old token is no longer valid, return an error
func (c *TokenClient) GetServiceAccountToken(namespace, name string, tr *authenticationv1.TokenRequest) (*authenticationv1.TokenRequest, error) {
	return c.manager.GetServiceAccountToken(namespace, name, tr)
}

// DeleteServiceAccountToken should be invoked when pod got deleted. It simply
// clean token manager cache.
func (c *TokenClient) DeleteServiceAccountToken(podUID types.UID) {
	c.manager.DeleteServiceAccountToken(podUID)
}
