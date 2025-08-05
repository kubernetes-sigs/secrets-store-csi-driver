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

package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

// Validate ensures the payload file paths are well formatted.
func Validate(payloads []*v1alpha1.File) error {
	for i := range payloads {
		if err := validatePath(payloads[i].Path); err != nil {
			return err
		}
		if filepath.Clean(payloads[i].Path) != payloads[i].Path {
			return fmt.Errorf("invalid filepath: %q", payloads[i].Path)
		}
	}

	return nil
}

// WritePayloads writes the files to target directory. This helper builds the
// atomic writer and converts the v1alpha1.File proto to the FileProjection type
// used by the atomic writer.
func WritePayloads(path string, payloads []*v1alpha1.File, gid int64) error {
	if err := Validate(payloads); err != nil {
		return err
	}

	// cleanup any payload paths that may have been written by a previous
	// version of the driver/provider.
	if err := cleanupProviderFiles(path, payloads); err != nil {
		return fmt.Errorf("cleanup failure: %w", err)
	}

	w, err := NewAtomicWriter(path, "secrets-store-csi-driver")
	if err != nil {
		return err
	}

	// convert v1alpha1.File to FileProjection
	files := make(map[string]FileProjection, len(payloads))
	for _, payload := range payloads {
		files[payload.GetPath()] = FileProjection{
			Data:    payload.GetContents(),
			Mode:    payload.GetMode(),
			FsGroup: &gid,
		}
	}

	return w.Write(files)
}

// cleanupProviderFiles checks all the paths from payloads to determine whether
// they are a symlink. If the path is not a symlink then it is likely that the
// provider wrote the file to the mount directly instead of using the
// atomic_writer.
//
// To ensure a seamless upgrade from the old style of mounted file to the
// atomic_writer style, these files need to be deleted otherwise they will not
// get updated
func cleanupProviderFiles(path string, payloads []*v1alpha1.File) error {
	for i := range payloads {
		// AtomicWriter only symlinks the top file or directory
		firstComponent := strings.Split(payloads[i].GetPath(), string(os.PathSeparator))[0]

		p := filepath.Join(path, firstComponent)
		info, err := os.Lstat(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		// skip symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}
