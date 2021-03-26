/*
Copyright 2016 The Kubernetes Authors.
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

// Forked from kubernetes/pkg/volume/util/atomic_writer.go
//  * tag: v1.20.5,
//  * commit: 6b1d87acf3c8253c123756b9e61dac642678305f
//  * link: https://github.com/kubernetes/kubernetes/blob/6b1d87acf3c8253c123756b9e61dac642678305f/pkg/volume/util/atomic_writer.go
//
// Borrows the file/path validation. This does not write files atomically
// as described in https://github.com/kubernetes/kubernetes/issues/18372 so
// behavior may be different from watching or reloading
// files when compared to K8S native Secrets.

package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

const (
	maxFileNameLength = 255
	maxPathLength     = 4096
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

// WritePayloads writes the files to target directory. One of the main
// deviations from atomic_writer.go is that this does not attempt to be atomic
// in updates. atomic_writer.go relies upon ../data folder, but in CSI driver
// context that path is outside of the tmpfs.
func WritePayloads(path string, payloads []*v1alpha1.File) error {
	for _, payload := range payloads {
		content := payload.Contents
		mode := os.FileMode(payload.Mode)
		fullPath := filepath.Join(path, payload.Path)
		baseDir, _ := filepath.Split(fullPath)

		if err := os.MkdirAll(baseDir, os.ModePerm); err != nil {
			return fmt.Errorf("unable to create directory %s: %w", baseDir, err)
		}

		if err := os.WriteFile(fullPath, content, mode); err != nil {
			return fmt.Errorf("unable to write file %s with mode %v: %w", fullPath, mode, err)
		}

		// Chmod is needed because ioutil.WriteFile() ends up calling
		// open(2) to create the file, so the final mode used is "mode &
		// ~umask". But we want to make sure the specified mode is used
		// in the file no matter what the umask is.
		if err := os.Chmod(fullPath, mode); err != nil {
			return fmt.Errorf("unable to change file %s with mode %v: %w", fullPath, mode, err)
		}
	}
	return nil
}

// validatePath validates a single path, returning an error if the path is
// invalid.  paths may not:
//
// 1. be absolute
// 2. contain '..' as an element
// 3. start with '..'
// 4. contain filenames larger than 255 characters
// 5. be longer than 4096 characters
func validatePath(targetPath string) error {
	// TODO: somehow unify this with the similar api validation,
	// validateVolumeSourcePath; the error semantics are just different enough
	// from this that it was time-prohibitive trying to find the right
	// refactoring to re-use.
	if targetPath == "" {
		return fmt.Errorf("invalid path: must not be empty: %q", targetPath)
	}
	if filepath.IsAbs(targetPath) {
		return fmt.Errorf("invalid path: must be relative path: %s", targetPath)
	}

	if len(targetPath) > maxPathLength {
		return fmt.Errorf("invalid path: must be less than or equal to %d characters", maxPathLength)
	}

	items := strings.Split(targetPath, string(os.PathSeparator))
	for _, item := range items {
		if item == ".." {
			return fmt.Errorf("invalid path: must not contain '..': %s", targetPath)
		}
		if len(item) > maxFileNameLength {
			return fmt.Errorf("invalid path: filenames must be less than or equal to %d characters", maxFileNameLength)
		}
	}
	if strings.HasPrefix(items[0], "..") && len(items[0]) > 2 {
		return fmt.Errorf("invalid path: must not start with '..': %s", targetPath)
	}

	return nil
}
