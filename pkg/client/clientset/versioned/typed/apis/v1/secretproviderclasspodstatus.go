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

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
	scheme "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned/scheme"
)

// SecretProviderClassPodStatusesGetter has a method to return a SecretProviderClassPodStatusInterface.
// A group's client should implement this interface.
type SecretProviderClassPodStatusesGetter interface {
	SecretProviderClassPodStatuses(namespace string) SecretProviderClassPodStatusInterface
}

// SecretProviderClassPodStatusInterface has methods to work with SecretProviderClassPodStatus resources.
type SecretProviderClassPodStatusInterface interface {
	Create(ctx context.Context, secretProviderClassPodStatus *v1.SecretProviderClassPodStatus, opts metav1.CreateOptions) (*v1.SecretProviderClassPodStatus, error)
	Update(ctx context.Context, secretProviderClassPodStatus *v1.SecretProviderClassPodStatus, opts metav1.UpdateOptions) (*v1.SecretProviderClassPodStatus, error)
	UpdateStatus(ctx context.Context, secretProviderClassPodStatus *v1.SecretProviderClassPodStatus, opts metav1.UpdateOptions) (*v1.SecretProviderClassPodStatus, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.SecretProviderClassPodStatus, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.SecretProviderClassPodStatusList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.SecretProviderClassPodStatus, err error)
	SecretProviderClassPodStatusExpansion
}

// secretProviderClassPodStatuses implements SecretProviderClassPodStatusInterface
type secretProviderClassPodStatuses struct {
	client rest.Interface
	ns     string
}

// newSecretProviderClassPodStatuses returns a SecretProviderClassPodStatuses
func newSecretProviderClassPodStatuses(c *SecretsstoreV1Client, namespace string) *secretProviderClassPodStatuses {
	return &secretProviderClassPodStatuses{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the secretProviderClassPodStatus, and returns the corresponding secretProviderClassPodStatus object, and an error if there is any.
func (c *secretProviderClassPodStatuses) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.SecretProviderClassPodStatus, err error) {
	result = &v1.SecretProviderClassPodStatus{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("secretproviderclasspodstatuses").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of SecretProviderClassPodStatuses that match those selectors.
func (c *secretProviderClassPodStatuses) List(ctx context.Context, opts metav1.ListOptions) (result *v1.SecretProviderClassPodStatusList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.SecretProviderClassPodStatusList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("secretproviderclasspodstatuses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested secretProviderClassPodStatuses.
func (c *secretProviderClassPodStatuses) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("secretproviderclasspodstatuses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a secretProviderClassPodStatus and creates it.  Returns the server's representation of the secretProviderClassPodStatus, and an error, if there is any.
func (c *secretProviderClassPodStatuses) Create(ctx context.Context, secretProviderClassPodStatus *v1.SecretProviderClassPodStatus, opts metav1.CreateOptions) (result *v1.SecretProviderClassPodStatus, err error) {
	result = &v1.SecretProviderClassPodStatus{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("secretproviderclasspodstatuses").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(secretProviderClassPodStatus).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a secretProviderClassPodStatus and updates it. Returns the server's representation of the secretProviderClassPodStatus, and an error, if there is any.
func (c *secretProviderClassPodStatuses) Update(ctx context.Context, secretProviderClassPodStatus *v1.SecretProviderClassPodStatus, opts metav1.UpdateOptions) (result *v1.SecretProviderClassPodStatus, err error) {
	result = &v1.SecretProviderClassPodStatus{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("secretproviderclasspodstatuses").
		Name(secretProviderClassPodStatus.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(secretProviderClassPodStatus).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *secretProviderClassPodStatuses) UpdateStatus(ctx context.Context, secretProviderClassPodStatus *v1.SecretProviderClassPodStatus, opts metav1.UpdateOptions) (result *v1.SecretProviderClassPodStatus, err error) {
	result = &v1.SecretProviderClassPodStatus{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("secretproviderclasspodstatuses").
		Name(secretProviderClassPodStatus.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(secretProviderClassPodStatus).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the secretProviderClassPodStatus and deletes it. Returns an error if one occurs.
func (c *secretProviderClassPodStatuses) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("secretproviderclasspodstatuses").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *secretProviderClassPodStatuses) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("secretproviderclasspodstatuses").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched secretProviderClassPodStatus.
func (c *secretProviderClassPodStatuses) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.SecretProviderClassPodStatus, err error) {
	result = &v1.SecretProviderClassPodStatus{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("secretproviderclasspodstatuses").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
