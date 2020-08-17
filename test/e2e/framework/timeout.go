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

package framework

import (
	"time"
)

const (
	CreateTimeout = 10 * time.Second
	CreatePolling = 1 * time.Second

	DeleteTimeout = 10 * time.Second
	DeletePolling = 1 * time.Second

	ListTimeout = 10 * time.Second
	ListPolling = 1 * time.Second

	GetTimeout = 1 * time.Minute
	GetPolling = 5 * time.Second

	UpdateTimeout = 10 * time.Second
	UpdatePolling = 1 * time.Second

	WaitTimeout = 5 * time.Minute
	WaitPolling = 5 * time.Second

	HelmTimeout = time.Duration(15) * time.Minute
)
