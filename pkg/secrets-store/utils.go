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

package secretsstore

import (
	"os"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func (cs *controllerServer) findVolumeByName(volName string) (csi.Volume, bool) {
	return cs.findVolume("name", volName)
}

func (cs *controllerServer) findVolumeByID(volID string) (csi.Volume, bool) {
	return cs.findVolume("id", volID)
}

func (cs *controllerServer) addVolume(name string, vol csi.Volume) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.vols[name] = vol
}

func (cs *controllerServer) findVolume(key, nameOrID string) (csi.Volume, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	return cs.findVolumeInternal(key, nameOrID)
}

func (cs *controllerServer) findVolumeInternal(key, nameOrID string) (csi.Volume, bool) {
	switch key {
	case "name":
		vol, ok := cs.vols[nameOrID]
		return vol, ok

	case "id":
		for _, vol := range cs.vols {
			if strings.EqualFold(nameOrID, vol.VolumeId) {
				return vol, true
			}
		}
	}
	return csi.Volume{}, false
}

func getProvidersVolumePath() string {
	return os.Getenv("PROVIDERS_VOLUME_PATH")
}

func isMockProvider(provider string) bool {
	return strings.EqualFold(provider, "mock_provider")
}
