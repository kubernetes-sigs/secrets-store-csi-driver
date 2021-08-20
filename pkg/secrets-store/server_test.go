/*
Copyright 2021 The Kubernetes Authors.

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

package secretsstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseEndpointNoError(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		wantSockType string
		wantAddr     string
	}{
		{
			name:         "valid unix domain socket endpoint",
			endpoint:     "unix://fake.sock",
			wantSockType: "unix",
			wantAddr:     "fake.sock",
		},
		{
			name:         "valid nested unix domain socket endpoint",
			endpoint:     "unix:///fakedir/fakedir/fake.sock",
			wantSockType: "unix",
			wantAddr:     "/fakedir/fakedir/fake.sock",
		},
		{
			name:         "valid tcp endpoint",
			endpoint:     "tcp://127.0.0.1:1234",
			wantSockType: "tcp",
			wantAddr:     "127.0.0.1:1234",
		},
		{
			name:         "valid tcp endpoint with uppercase",
			endpoint:     "TCP://127.0.0.1:1234",
			wantSockType: "TCP",
			wantAddr:     "127.0.0.1:1234",
		},
		{
			name:         "valid tcp endpoint with hostname",
			endpoint:     "tcp://example.com:1234",
			wantSockType: "tcp",
			wantAddr:     "example.com:1234",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sockType, addr, err := parseEndpoint(test.endpoint)
			assert.NoError(t, err)
			assert.Equal(t, test.wantSockType, sockType)
			assert.Equal(t, test.wantAddr, addr)
		})
	}
}

func TestParseEndpointError(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{
			name:     "invalid endpoint",
			endpoint: "unix:/fake.sock/",
		},
		{
			name:     "socket type not provided",
			endpoint: "fake.sock",
		},
		{
			name:     "socket path incomplete",
			endpoint: "unix://",
		},
		{
			name:     "socket path incomplete and type not provided",
			endpoint: "://",
		},
		{
			name:     "empty endpoint",
			endpoint: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := parseEndpoint(test.endpoint)
			assert.Error(t, err)
		})
	}
}
