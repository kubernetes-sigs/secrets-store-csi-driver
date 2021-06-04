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

// Package fileutil includes helpers for dealing with CSI mount paths and reading/writing files.
package fileutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	targetPathRe = regexp.MustCompile(`[\\|\/]+pods[\\|\/]+(.+?)[\\|\/]+volumes[\\|\/]+kubernetes.io~csi[\\|\/]+(.+?)[\\|\/]+mount$`)
)

// GetMountedFiles returns all the mounted files mapping their path relative to
// targetPath to the absolute paths.
//
// This will filter out files by atomic_writer (which reserves file prefixed
// `..` and follows the symlinks created by atomic_writer).
func GetMountedFiles(targetPath string) (map[string]string, error) {
	paths := make(map[string]string)

	// visitor is called for each file walked.
	// root is the root that will be walked, and base is the starting path
	// relative to targetPath when walking began
	visitor := func(root, base string) filepath.WalkFunc {
		return func(path string, info os.FileInfo, err error) error {
			// if there was an error walking path immediately propagate it
			if err != nil {
				return err
			}

			// do not include directories in result
			if info.IsDir() {
				return nil
			}

			// find the relative path of the root that was walked
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}

			paths[filepath.Join(base, rel)] = path
			return nil
		}
	}

	// atomic writer will write data to, but filepath.Walk does not follow
	// symlinks. follow any symlinks in the targetPath and then walk every item.
	d, err := os.ReadDir(targetPath)
	if err != nil {
		return nil, err
	}

	// for each file/directory in the targetPath, walk that entry
	for _, entry := range d {
		// skip the reserved paths of targetPath/..*
		if strings.HasPrefix(entry.Name(), "..") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		// the path to the file relative to targetPath
		// i.e. foo
		base := info.Name()

		// the path before following the symlink
		// i.e. targetPath/foo
		root := filepath.Join(targetPath, base)

		// for symlinks in the targetPath...
		if info.Mode()&os.ModeSymlink != 0 {
			// the resolved relative path
			// i.e. ..data/foo
			actual, err := os.Readlink(root)
			if err != nil {
				return nil, err
			}

			// update the root path to walk to be after following the symlink
			// i.e. targetPath/..data/foo
			root = filepath.Join(targetPath, actual)
		}

		if err := filepath.Walk(root, visitor(root, base)); err != nil {
			return paths, err
		}
	}
	return paths, nil
}

// GetPodUIDFromTargetPath returns podUID from targetPath
func GetPodUIDFromTargetPath(targetPath string) string {
	match := targetPathRe.FindStringSubmatch(targetPath)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

// GetVolumeNameFromTargetPath returns volumeName from targetPath
func GetVolumeNameFromTargetPath(targetPath string) string {
	match := targetPathRe.FindStringSubmatch(targetPath)
	if len(match) < 2 {
		return ""
	}
	return match[2]
}
