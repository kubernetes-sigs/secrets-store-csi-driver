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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	targetPathRe = regexp.MustCompile(`[\\|\/]+pods[\\|\/]+(.+?)[\\|\/]+volumes[\\|\/]+kubernetes.io~csi[\\|\/]+(.+?)[\\|\/]+mount$`)
)

// GetMountedFiles returns all the mounted files names with filepath base as key
func GetMountedFiles(targetPath string) (map[string]string, error) {
	paths := make(map[string]string)
	// loop thru all the mounted files
	var files []string
	err := filepath.Walk(targetPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		relpath, err := filepath.Rel(targetPath, path)
		if err != nil {
			return err
		}
		files = append(files, relpath)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list all files in target path %s, err: %v", targetPath, err)
	}

	sep := "/"
	if strings.HasPrefix(targetPath, "c:\\") {
		sep = "\\"
	} else if strings.HasPrefix(targetPath, `c:\`) {
		sep = `\`
	}
	for _, file := range files {
		paths[file] = targetPath + sep + file
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
