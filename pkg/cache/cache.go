/*
Copyright 2018 The Kubernetes Authors.

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

package cache

import (
	"fmt"
	"time"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/cache/internal"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	crcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var log = ctrl.Log.WithName("object-cache")

// Options are the optional arguments for creating a new InformersMap object
type Options struct {
	crcache.Options
	// FieldSelectorByResource restricts the cache's ListWatch to the resources with desired field
	// Default watches resources with any field
	FieldSelectorByResource map[schema.GroupResource]string

	// LabelSelectorByResource restricts the cache's ListWatch to the resources with desired label
	// Default watches resources with any label
	LabelSelectorByResource map[schema.GroupResource]string
}

var defaultResyncTime = 10 * time.Hour

// New initializes and returns a new Cache.
func New(config *rest.Config, opts Options) (crcache.Cache, error) {
	opts, err := defaultOpts(config, opts)
	if err != nil {
		return nil, err
	}
	im := internal.NewInformersMap(config, opts.Scheme, opts.Mapper, *opts.Resync, opts.Namespace, opts.FieldSelectorByResource, opts.LabelSelectorByResource)
	return &informerCache{InformersMap: im}, nil
}

func Builder(opts Options) crcache.NewCacheFunc {
	return func(config *rest.Config, cropts crcache.Options) (crcache.Cache, error) {
		opts.Options = cropts
		return New(config, opts)
	}
}

func defaultOpts(config *rest.Config, opts Options) (Options, error) {
	// Use the default Kubernetes Scheme if unset
	if opts.Scheme == nil {
		opts.Scheme = scheme.Scheme
	}

	// Construct a new Mapper if unset
	if opts.Mapper == nil {
		var err error
		opts.Mapper, err = apiutil.NewDiscoveryRESTMapper(config)
		if err != nil {
			log.WithName("setup").Error(err, "Failed to get API Group-Resources")
			return opts, fmt.Errorf("could not create RESTMapper from config")
		}
	}

	// Default the resync period to 10 hours if unset
	if opts.Resync == nil {
		opts.Resync = &defaultResyncTime
	}
	return opts, nil
}
