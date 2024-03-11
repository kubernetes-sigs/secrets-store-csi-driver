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

	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/secrets-store-csi-driver/secret-sync-controller/pkg/k8s/token"
)

// TokenClient is a client for Kubernetes Token API
type TokenClient struct {
	manager *token.Manager
}

// NewTokenClient creates a new TokenClient
// The client will be used to request a token for token audiences configured in the Secret Sync Controller.
func NewTokenClient(kubeClient kubernetes.Interface) *TokenClient {
	return &TokenClient{
		manager: token.NewManager(kubeClient),
	}
}

// SecretProviderServiceAccountTokenAttrs returns the token for the federated service account that can be bound to the pod.
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
func (c *TokenClient) SecretProviderServiceAccountTokenAttrs(namespace, serviceAccountName string, audiences []string) (map[string]string, error) {

	if len(audiences) == 0 {
		return nil, nil
	}

	outputs := map[string]authenticationv1.TokenRequestStatus{}
	var tokenExpirationSeconds int64 = 600

	for _, aud := range audiences {
		tr := &authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				ExpirationSeconds: &tokenExpirationSeconds,
				Audiences:         []string{aud},
			},
		}

		tr, err := c.GetServiceAccountToken(namespace, serviceAccountName, tr)
		if err != nil {
			return nil, err
		}
		outputs[aud] = tr.Status
	}

	klog.V(5).InfoS("Fetched service account token attrs", "serviceAccountName", serviceAccountName, "namespace", namespace)
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
