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
	"fmt"
	"io/ioutil"
	"strings"
)

// GetMountedFiles returns all the mounted files names with filepath base as key
func GetMountedFiles(targetPath string) (map[string]string, error) {
	paths := make(map[string]string)
	// loop thru all the mounted files
	files, err := ioutil.ReadDir(targetPath)
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
		paths[file.Name()] = targetPath + sep + file.Name()
	}
	return paths, nil
}
