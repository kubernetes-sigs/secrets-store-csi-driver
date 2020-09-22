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

package fileutil

import (
	"io/ioutil"
	"testing"
)

func TestGetMountedFiles(t *testing.T) {
	tests := []struct {
		name        string
		targetPath  func() string
		expectedErr bool
	}{
		{
			name:        "target path not found",
			targetPath:  func() string { return "" },
			expectedErr: true,
		},
		{
			name: "target path dir found",
			targetPath: func() string {
				tmpDir, err := ioutil.TempDir("", "ut")
				if err != nil {
					t.Errorf("failed to created tmp file, err: %+v", err)
					return ""
				}
				return tmpDir
			},
			expectedErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := GetMountedFiles(test.targetPath())
			if test.expectedErr != (err != nil) {
				t.Fatalf("expected err: %v, got: %+v", test.expectedErr, err)
			}
		})
	}
}
