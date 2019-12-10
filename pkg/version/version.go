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
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/blang/semver"
	log "github.com/sirupsen/logrus"
)

// providerVersion holds current provider version
type providerVersion struct {
	// Version is the current provider version
	Version string `json:"version"`
	// BuildDate is the date provider binary was built
	BuildDate string `json:"buildDate"`
	// MinDriverVersion is minimum driver version the provider works with
	// this can be used later for bidirectional compatibility checks between driver-provider
	MinDriverVersion string `json:"minDriverVersion"`
}

// IsProviderCompatible checks if the provider version is compatible with
// current driver version.
func IsProviderCompatible(provider string, minProviderVersion string) (bool, error) {
	// get current provider version
	currProviderVersion, err := getProviderVersion(provider)
	if err != nil {
		return false, err
	}
	return isProviderCompatible(currProviderVersion, minProviderVersion)
}

// GetMinimumProviderVersions creates a map with provider name and minimum version
// supported with this driver.
func GetMinimumProviderVersions(minProviderVersions string) (map[string]string, error) {
	providerVersionMap := make(map[string]string)

	if minProviderVersions == "" {
		return providerVersionMap, nil
	}

	// splitting on , delimiter will result in array of provider=value string
	providers := strings.Split(minProviderVersions, ",")
	for _, provider := range providers {
		provider = strings.TrimSpace(provider)

		pv := strings.Split(provider, "=")
		// validate its expected format provider=version
		if len(pv) != 2 || len(pv[0]) == 0 || len(pv[1]) == 0 {
			return providerVersionMap, fmt.Errorf("min provider version not defined in expected format, got %+v", pv)
		}

		providerVersionMap[strings.TrimSpace(pv[0])] = strings.TrimSpace(pv[1])
	}

	log.Debugf("Minimum supported provider versions: %v", providerVersionMap)
	return providerVersionMap, nil
}

func getProviderVersion(providerName string) (string, error) {
	cmd := exec.Command(providerName, "--version")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stderr, cmd.Stdout = stderr, stdout

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error getting current provider version for %s, err: %v, output: %v", providerName, err, stderr.String())
	}
	var pv providerVersion
	if err := json.Unmarshal(stdout.Bytes(), &pv); err != nil {
		return "", fmt.Errorf("error unmarshalling provider version %v", err)
	}

	log.Debugf("provider: %s, version %s, build date: %s", providerName, pv.Version, pv.BuildDate)
	return pv.Version, nil
}

func isProviderCompatible(currVersion, minVersion string) (bool, error) {
	currV, err := semver.Make(currVersion)
	if err != nil {
		return false, err
	}
	minV, err := semver.Make(minVersion)
	if err != nil {
		return false, err
	}
	return currV.Compare(minV) >= 0, nil
}
