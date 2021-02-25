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

// Forked from kubernetes/pkg/volume/util/atomic_writer_test.go
//  * tag: v1.20.5,
//  * commit: 6b1d87acf3c8253c123756b9e61dac642678305f
//  * link: https://github.com/kubernetes/kubernetes/blob/6b1d87acf3c8253c123756b9e61dac642678305f/pkg/volume/util/atomic_writer_test.go

package fileutil

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/test_utils/tmpdir"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

func TestValidatePath_Success(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{
			name: "valid 1",
			path: "i/am/well/behaved.txt",
		},
		{
			name: "valid 2",
			path: "keepyourheaddownandfollowtherules.txt",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validatePath(tc.path); err != nil {
				t.Errorf("unexpected failure: %v", err)
			}
		})
	}
}

func TestValidatePath_Error(t *testing.T) {
	maxPath := strings.Repeat("a", maxPathLength+1)
	maxFile := strings.Repeat("a", maxFileNameLength+1)

	cases := []struct {
		name string
		path string
	}{
		{
			name: "max path length",
			path: maxPath,
		},
		{
			name: "max file length",
			path: maxFile,
		},
		{
			name: "absolute failure",
			path: "/dev/null",
		},
		{
			name: "reserved path",
			path: "..sneaky.txt",
		},
		{
			name: "contains doubledot 1",
			path: "hello/there/../../../../../../etc/passwd",
		},
		{
			name: "contains doubledot 2",
			path: "hello/../etc/somethingbad",
		},
		{
			name: "empty",
			path: "",
		},
		{
			name: "parent",
			path: "..",
		},
		{
			name: "sibling",
			path: "../sibling",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if validatePath(tc.path) == nil {
				t.Error("unexpected success")
			}
		})
	}
}

func TestValidate_Success(t *testing.T) {
	cases := []struct {
		name    string
		payload []*v1alpha1.File
	}{
		{
			name: "valid double payload",
			payload: []*v1alpha1.File{
				{
					Path: "foo",
				},
				{
					Path: "bar",
				},
			},
		},
		{
			name: "valid single payload",
			payload: []*v1alpha1.File{
				{
					Path: "foo",
				},
			},
		},
		{
			name: "valid nested path",
			payload: []*v1alpha1.File{
				{
					Path: "foo/bar",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.payload); err != nil {
				t.Errorf("%v: unexpected error: %v", tc.name, err)
			}
		})
	}
}

func TestValidate_Error(t *testing.T) {
	maxPath := strings.Repeat("a", maxPathLength+1)

	cases := []struct {
		name    string
		payload []*v1alpha1.File
	}{
		{
			name: "payload with path length > 4096 is invalid",
			payload: []*v1alpha1.File{
				{
					Path: maxPath,
				},
			},
		},
		{
			name: "payload with absolute path is invalid",
			payload: []*v1alpha1.File{
				{
					Path: "/dev/null",
				},
			},
		},
		{
			name: "payload with reserved path is invalid",
			payload: []*v1alpha1.File{
				{
					Path: "..sneaky.txt",
				},
			},
		},
		{
			name: "payload with doubledot path is invalid",
			payload: []*v1alpha1.File{
				{
					Path: "foo/../etc/password",
				},
			},
		},
		{
			name: "payload with empty path is invalid",
			payload: []*v1alpha1.File{
				{
					Path: "",
				},
			},
		},
		{
			name: "payload with unclean path should be cleaned",
			payload: []*v1alpha1.File{
				{
					Path: "foo////bar",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if Validate(tc.payload) == nil {
				t.Error("unexpected success")
			}
		})
	}
}

func TestWritePayloads(t *testing.T) {
	cases := []struct {
		name    string
		payload []*v1alpha1.File
	}{
		{
			name: "simple payload",
			payload: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0644,
					Contents: []byte("foo"),
				},
			},
		},
		{
			name: "simple payload - mode",
			payload: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0440,
					Contents: []byte("foo"),
				},
			},
		},
		{
			name: "simple payload - mode2",
			payload: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0777,
					Contents: []byte("foo"),
				},
			},
		},
		{
			name: "simple payload - mode3",
			payload: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0666,
					Contents: []byte("foo"),
				},
			},
		},
		{
			name: "nested path payload",
			payload: []*v1alpha1.File{
				{
					Path:     "foo/bar",
					Mode:     0644,
					Contents: []byte("foo"),
				},
			},
		},
		{
			name: "multiple nested paths",
			payload: []*v1alpha1.File{
				{
					Path:     "foo/bar/baz",
					Mode:     0644,
					Contents: []byte("foo"),
				},
			},
		},
		{
			name: "multiple nested paths and files",
			payload: []*v1alpha1.File{
				{
					Path:     "foo/bar/baz",
					Mode:     0644,
					Contents: []byte("foo"),
				},
				{
					Path:     "foo/bar/2.txt",
					Mode:     0644,
					Contents: []byte("two"),
				},
				{
					Path:     "foo/1.txt",
					Mode:     0644,
					Contents: []byte("one"),
				},
				{
					Path:     "root.txt",
					Mode:     0644,
					Contents: []byte("root"),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := tmpdir.New(t, "", "ut")

			if err := WritePayloads(dir, tc.payload); err != nil {
				t.Errorf("WritePayload() got error: %v", err)
			}

			if err := readPayloads(dir, tc.payload); err != nil {
				t.Errorf("WritePayload() could not be read: %v", err)
			}
		})
	}
}

func readPayloads(path string, payloads []*v1alpha1.File) error {
	for _, p := range payloads {
		fp := filepath.Join(path, p.Path)
		info, err := os.Stat(fp)
		if err != nil {
			return err
		}
		if info.Mode() != fs.FileMode(p.Mode) {
			return fmt.Errorf("unexpected file mode. got: %v, want: %v", info.Mode(), fs.FileMode(p.Mode))
		}
		contents, err := os.ReadFile(fp)
		if err != nil {
			return err
		}
		if !bytes.Equal(contents, p.Contents) {
			return errors.New("missmatched file contents")
		}
	}
	return nil
}
