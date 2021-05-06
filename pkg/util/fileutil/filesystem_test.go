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
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"sigs.k8s.io/secrets-store-csi-driver/pkg/test_utils/tmpdir"
)

func TestGetMountedFiles(t *testing.T) {
	tests := []struct {
		name        string
		targetPath  func(t *testing.T) string
		expectedErr bool
		want        []string
	}{
		{
			name:        "target path not found",
			targetPath:  func(t *testing.T) string { return "" },
			expectedErr: true,
		},
		{
			name: "target path dir found",
			targetPath: func(t *testing.T) string {
				return tmpdir.New(t, "", "ut")
			},
			expectedErr: false,
		},
		{
			name: "target path dir/file found",
			targetPath: func(t *testing.T) string {
				dir := tmpdir.New(t, "", "ut")
				f, err := os.Create(filepath.Join(dir, "secret.txt"))
				if err != nil {
					t.Fatalf("error writing file: %s", err)
				}
				if err := f.Close(); err != nil {
					t.Fatalf("error writing file: %s", err)
				}
				return dir
			},
			expectedErr: false,
			want:        []string{"secret.txt"},
		},
		{
			name: "target path dir/dir/file found",
			targetPath: func(t *testing.T) string {
				dir := tmpdir.New(t, "", "ut")
				if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0700); err != nil {
					t.Fatalf("could not make subdir: %s", err)
				}
				f, err := os.Create(filepath.Join(dir, "subdir", "secret.txt"))
				if err != nil {
					t.Fatalf("could not write file: %s", err)
				}
				if err := f.Close(); err != nil {
					t.Fatalf("error writing file: %s", err)
				}
				return dir
			},
			expectedErr: false,
			want:        []string{filepath.Join("subdir", "secret.txt")},
		},
		{
			name: "target path with atomic_writer symlinks",
			targetPath: func(t *testing.T) string {
				dir := tmpdir.New(t, "", "ut")
				writer, err := NewAtomicWriter(dir, "test")
				if err != nil {
					t.Fatalf("unable to create AtomicWriter: %s", err)
				}
				err = writer.Write(map[string]FileProjection{
					"foo/bar.txt": {
						Data: []byte("foo"),
						Mode: 0700,
					},
					"foo/baz.txt": {
						Data: []byte("baz"),
						Mode: 0700,
					},
					"foo.txt": {
						Data: []byte("foo.txt"),
						Mode: 0700,
					},
				})
				if err != nil {
					t.Fatalf("unable to write FileProjection: %s", err)
				}
				return dir
			},
			expectedErr: false,
			want: []string{
				"foo.txt",
				filepath.Join("foo", "bar.txt"),
				filepath.Join("foo", "baz.txt"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := GetMountedFiles(test.targetPath(t))
			if test.expectedErr != (err != nil) {
				t.Fatalf("expected err: %v, got: %+v", test.expectedErr, err)
			}

			gotKeys := []string{}
			for k := range got {
				gotKeys = append(gotKeys, k)
			}
			sort.Strings(gotKeys)

			if diff := cmp.Diff(test.want, gotKeys, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("GetMountedFiles() keys mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetPodUIDFromTargetPath(t *testing.T) {
	cases := []struct {
		targetPath string
		want       string
	}{
		{
			targetPath: "/var/lib/kubelet/pods/7e7686a1-56c4-4c67-a6fd-4656ac484f0a/volumes/",
			want:       "",
		},
		{
			targetPath: "/var/lib/kubelet/pods/7e7686a1-56c4-4c67-a6fd-4656ac484f0a/volumes/kubernetes.io~csi",
			want:       "",
		},
		{
			targetPath: "/var/lib/kubelet/pods/7e7686a1-56c4-4c67-a6fd-4656ac484f0a/volumes/kubernetes.io~pv/pvvol/mount",
			want:       "",
		},
		{
			targetPath: "/var/lib/kubelet/pods/7e7686a1-56c4-4c67-a6fd-4656ac484f0a/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			want:       "7e7686a1-56c4-4c67-a6fd-4656ac484f0a",
		},
		{
			targetPath: `c:\var\lib\kubelet\pods\d4fd876f-bdb3-11e9-a369-0a5d188d99c0\volumes`,
			want:       "",
		},
		{
			targetPath: `c:\var\lib\kubelet\pods\d4fd876f-bdb3-11e9-a369-0a5d188d99c0\volumes\kubernetes.io~csi`,
			want:       "",
		},
		{
			targetPath: `c:\var\lib\kubelet\pods\d4fd876f-bdb3-11e9-a369-0a5d188d99c0\volumes\kubernetes.io~pv\pvvol\mount`,
			want:       "",
		},
		{
			targetPath: `c:\var\lib\kubelet\pods\d4fd876f-bdb3-11e9-a369-0a5d188d99c0\volumes\kubernetes.io~csi\secrets-store-inline\mount`,
			want:       "d4fd876f-bdb3-11e9-a369-0a5d188d99c0",
		},
		{
			targetPath: `c:\\var\\lib\\kubelet\\pods\\d4fd876f-bdb3-11e9-a369-0a5d188d9934\\volumes`,
			want:       "",
		},
		{
			targetPath: `c:\\var\\lib\\kubelet\\pods\\d4fd876f-bdb3-11e9-a369-0a5d188d9934\\volumes\\kubernetes.io~csi`,
			want:       "",
		},
		{
			targetPath: `c:\\var\\lib\\kubelet\\pods\\d4fd876f-bdb3-11e9-a369-0a5d188d9934\\volumes\\kubernetes.io~pv\\pvvol\\mount`,
			want:       "",
		},
		{
			targetPath: `c:\\var\\lib\\kubelet\\pods\\d4fd876f-bdb3-11e9-a369-0a5d188d9934\\volumes\\kubernetes.io~csi\\secrets-store-inline\\mount`,
			want:       "d4fd876f-bdb3-11e9-a369-0a5d188d9934",
		},
		{
			targetPath: "/var/lib/",
			want:       "",
		},
		{
			targetPath: "/var/lib/kubelet/pods",
			want:       "",
		},
		{
			targetPath: "/opt/new/var/lib/kubelet/pods/456457fc-d980-4191-b5eb-daf70c4ff7c1/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			want:       "456457fc-d980-4191-b5eb-daf70c4ff7c1",
		},
		{
			targetPath: "data/kubelet/pods/456457fc-d980-4191-b5eb-daf70c4ff7c1/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			want:       "456457fc-d980-4191-b5eb-daf70c4ff7c1",
		},
		{
			targetPath: "data/kubelet/pods/456457fc-d980-4191-b5eb-daf70c4ff7c1/volumes/kubernetes.io~pv/secrets-store-inline/mount",
			want:       "",
		},
		{
			targetPath: "/var/lib/kubelet/pods/7e7686a1-56c4-4c67-a6fd-4656ac484f0a/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			want:       "7e7686a1-56c4-4c67-a6fd-4656ac484f0a",
		},
		{
			targetPath: `c:\var\lib\kubelet\pods\d4fd876f-bdb3-11e9-a369-0a5d188d99c0\volumes\kubernetes.io~csi\secrets-store-inline\mount`,
			want:       "d4fd876f-bdb3-11e9-a369-0a5d188d99c0",
		},
		{
			targetPath: `c:\\var\\lib\\kubelet\\pods\\d4fd876f-bdb3-11e9-a369-0a5d188d9934\\volumes\\kubernetes.io~csi\\secrets-store-inline\\mount`,
			want:       "d4fd876f-bdb3-11e9-a369-0a5d188d9934",
		},
		{
			targetPath: "/opt/new/var/lib/kubelet/pods/456457fc-d980-4191-b5eb-daf70c4ff7c1/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			want:       "456457fc-d980-4191-b5eb-daf70c4ff7c1",
		},
		{
			targetPath: "data/kubelet/pods/456457fc-d980-4191-b5eb-daf70c4ff7c1/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			want:       "456457fc-d980-4191-b5eb-daf70c4ff7c1",
		},
		{
			targetPath: "/var/lib/kubelet/pods/64f9ffb2-409e-4c58-9ea8-2a7d21050ece/volumes/kubernetes.io~secret/server-token-npdwt",
			want:       "",
		},
		{
			targetPath: `\\pods\\fakePod\\volumes\\kubernetes.io~csi\\myvol\\mount`,
			want:       "fakePod",
		},
	}

	for _, tc := range cases {
		got := GetPodUIDFromTargetPath(tc.targetPath)
		if got != tc.want {
			t.Errorf("GetPodUIDFromTargetPath(%v) = %v, want %v", tc.targetPath, got, tc.want)
		}
	}
}

func TestGetVolumeNameFromTargetPath(t *testing.T) {
	cases := []struct {
		targetPath string
		want       string
	}{
		{
			targetPath: "/var/lib/kubelet/pods/7e7686a1-56c4-4c67-a6fd-4656ac484f0a/volumes/",
			want:       "",
		},
		{
			targetPath: "/var/lib/kubelet/pods/7e7686a1-56c4-4c67-a6fd-4656ac484f0a/volumes/kubernetes.io~csi",
			want:       "",
		},
		{
			targetPath: "/var/lib/kubelet/pods/7e7686a1-56c4-4c67-a6fd-4656ac484f0a/volumes/kubernetes.io~pv/pvvol/mount",
			want:       "",
		},
		{
			targetPath: "/var/lib/kubelet/pods/7e7686a1-56c4-4c67-a6fd-4656ac484f0a/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			want:       "secrets-store-inline",
		},
		{
			targetPath: `c:\var\lib\kubelet\pods\d4fd876f-bdb3-11e9-a369-0a5d188d99c0\volumes`,
			want:       "",
		},
		{
			targetPath: `c:\var\lib\kubelet\pods\d4fd876f-bdb3-11e9-a369-0a5d188d99c0\volumes\kubernetes.io~csi`,
			want:       "",
		},
		{
			targetPath: `c:\var\lib\kubelet\pods\d4fd876f-bdb3-11e9-a369-0a5d188d99c0\volumes\kubernetes.io~pv\pvvol\mount`,
			want:       "",
		},
		{
			targetPath: `c:\var\lib\kubelet\pods\d4fd876f-bdb3-11e9-a369-0a5d188d99c0\volumes\kubernetes.io~csi\secrets-store-inline\mount`,
			want:       "secrets-store-inline",
		},
		{
			targetPath: `c:\\var\\lib\\kubelet\\pods\\d4fd876f-bdb3-11e9-a369-0a5d188d9934\\volumes`,
			want:       "",
		},
		{
			targetPath: `c:\\var\\lib\\kubelet\\pods\\d4fd876f-bdb3-11e9-a369-0a5d188d9934\\volumes\\kubernetes.io~csi`,
			want:       "",
		},
		{
			targetPath: `c:\\var\\lib\\kubelet\\pods\\d4fd876f-bdb3-11e9-a369-0a5d188d9934\\volumes\\kubernetes.io~pv\\pvvol\\mount`,
			want:       "",
		},
		{
			targetPath: `c:\\var\\lib\\kubelet\\pods\\d4fd876f-bdb3-11e9-a369-0a5d188d9934\\volumes\\kubernetes.io~csi\\secrets-store-inline\\mount`,
			want:       "secrets-store-inline",
		},
		{
			targetPath: "/var/lib/",
			want:       "",
		},
		{
			targetPath: "/var/lib/kubelet/pods",
			want:       "",
		},
		{
			targetPath: "/opt/new/var/lib/kubelet/pods/456457fc-d980-4191-b5eb-daf70c4ff7c1/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			want:       "secrets-store-inline",
		},
		{
			targetPath: "data/kubelet/pods/456457fc-d980-4191-b5eb-daf70c4ff7c1/volumes/kubernetes.io~csi/secrets-store-inline/mount",
			want:       "secrets-store-inline",
		},
		{
			targetPath: "data/kubelet/pods/456457fc-d980-4191-b5eb-daf70c4ff7c1/volumes/kubernetes.io~pv/secrets-store-inline/mount",
			want:       "",
		},
	}

	for _, tc := range cases {
		got := GetVolumeNameFromTargetPath(tc.targetPath)
		if got != tc.want {
			t.Errorf("GetVolumeNameFromTargetPath(%v) = %v, want %v", tc.targetPath, got, tc.want)
		}
	}
}
