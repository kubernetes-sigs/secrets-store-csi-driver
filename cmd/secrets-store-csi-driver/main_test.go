/*
Copyright 2026 The Kubernetes Authors.

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

package main

import (
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestIsNetworkError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "unrelated error",
			err:  errors.New("failed to create controller"),
			want: false,
		},
		{
			name: "wrapped net.Error",
			err:  fmt.Errorf("could not create RESTMapper from config: %w", &net.DNSError{Err: "no such host", Name: "kubernetes.default.svc", IsNotFound: true}),
			want: true,
		},
		{
			name: "connection refused string",
			err:  errors.New("Get \"https://10.0.0.1:443/api\": dial tcp 10.0.0.1:443: connect: connection refused"),
			want: true,
		},
		{
			name: "no such host string",
			err:  errors.New("dial tcp: lookup kubernetes.default.svc: no such host"),
			want: true,
		},
		{
			name: "i/o timeout string",
			err:  errors.New("dial tcp 10.0.0.1:443: i/o timeout"),
			want: true,
		},
		{
			name: "context deadline exceeded string",
			err:  errors.New("Get \"https://10.0.0.1:443/api\": context deadline exceeded"),
			want: true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := isNetworkError(test.err)
			if got != test.want {
				t.Errorf("isNetworkError() = %v, want %v", got, test.want)
			}
		})
	}
}
