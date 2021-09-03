/*
Copyright 2019 The Kubernetes Authors.

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

package version

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func TestGetUserAgent(t *testing.T) {
	BuildTime = "Now"
	BuildVersion = "version"
	Vcs = "hash"
	controllerName := "controller"

	expected := fmt.Sprintf("csi-secrets-store/%s/%s (%s/%s) %s/%s", controllerName, BuildVersion, runtime.GOOS, runtime.GOARCH, Vcs, BuildTime)
	actual := GetUserAgent(controllerName)
	if !strings.EqualFold(expected, actual) {
		t.Fatalf("expected: %s, got: %s", expected, actual)
	}
}
