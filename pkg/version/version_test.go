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
	"bytes"
	"fmt"
	"io"
	"os"
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

func TestPrintVersion(t *testing.T) {
	BuildTime = "Now"
	BuildVersion = "version"
	Vcs = "hash"

	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := PrintVersion()

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outC <- strings.TrimSpace(buf.String())
	}()

	// back to normal state
	w.Close()
	os.Stdout = old // restoring the real stdout
	out := <-outC

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expected := `{"BuildVersion":"version","GitCommit":"hash","BuildDate":"Now"}`
	if !strings.EqualFold(out, expected) {
		t.Fatalf("PrintVersion() expected %s, got %s", expected, out)
	}
}
