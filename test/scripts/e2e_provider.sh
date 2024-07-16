#!/usr/bin/env bash

# Copyright 2021 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

echo "running e2e provider test"
make e2e-bootstrap e2e-mock-provider-container

# if YAML_TEST env var is true, then use e2e-deploy-manifest instead of e2e-helm-deploy
if [ "$IS_YAML_TEST" = "true" ]; then
  echo "Deploying driver using manifest"
  make e2e-deploy-manifest
else
  echo "Deploying driver using helm chart"
  make e2e-helm-deploy
fi

make e2e-provider-deploy e2e-provider
