# Copyright 2018 The Kubernetes Authors.
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

REGISTRY?=docker.io/deislabs
REGISTRY_NAME = $(shell echo $(REGISTRY) | sed "s/.azurecr.io//g")
GIT_COMMIT ?= $(shell git rev-parse HEAD)
IMAGE_NAME=secrets-store-csi
IMAGE_VERSION?=v0.0.14
E2E_IMAGE_VERSION = v0.1.0-e2e-$(GIT_COMMIT)
# Use a custom version for E2E tests if we are testing in CI
ifdef CI
override IMAGE_VERSION := $(E2E_IMAGE_VERSION)
endif
IMAGE_TAG=$(REGISTRY)/$(IMAGE_NAME):$(IMAGE_VERSION)
IMAGE_TAG_LATEST=$(REGISTRY)/$(IMAGE_NAME):latest
LDFLAGS?='-X sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store.vendorVersion=$(IMAGE_VERSION) -extldflags "-static"'
GO_FILES=$(shell go list ./... | grep -v /test/sanity)

.PHONY: all build image clean test-style

GO111MODULE ?= on
export GO111MODULE
DOCKER_CLI_EXPERIMENTAL = enabled
export GOPATH GOBIN GO111MODULE DOCKER_CLI_EXPERIMENTAL

HAS_GOLANGCI := $(shell command -v golangci-lint;)

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: build

test: test-style
	go test $(GO_FILES) -v
	go vet $(GO_FILES)
test-style: setup
	@echo "==> Running static validations and linters <=="
	# Setting timeout to 5m as deafult is 1m
	golangci-lint run --timeout=5m
sanity-test:
	go test -v ./test/sanity
build: setup
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags $(LDFLAGS) -o _output/secrets-store-csi ./cmd/secrets-store-csi-driver
build-windows: setup
	CGO_ENABLED=0 GOOS=windows go build -a -ldflags $(LDFLAGS) -o _output/secrets-store-csi.exe ./cmd/secrets-store-csi-driver
image:
	docker buildx build --no-cache --build-arg LDFLAGS=$(LDFLAGS) -t $(IMAGE_TAG) -f docker/Dockerfile --platform="linux/amd64" --output "type=docker,push=false" .
image-windows:
	docker buildx build --no-cache --build-arg LDFLAGS=$(LDFLAGS) -t $(IMAGE_TAG) -f docker/windows.Dockerfile --platform="windows/amd64" --output "type=docker,push=false" .
clean:
	-rm -rf _output
setup: clean
	@echo "Setup..."
	$Q go env

ifndef HAS_GOLANGCI
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin v1.30.0
endif

.PHONY: mod
mod:
	@go mod tidy

KIND_VERSION ?= 0.8.1
KUBERNETES_VERSION ?= 1.18.2
VAULT_VERSION ?= 1.4.2

# USED for windows CI tests
.PHONY: e2e-test
e2e-test: e2e-bootstrap
	make e2e-azure

.PHONY: e2e-bootstrap
e2e-bootstrap: install-helm
	apt-get update && apt-get install bats && apt-get install gettext-base -y
	# Download and install Vault
	vault -v | grep -q v$(VAULT_VERSION) || (curl -LO https://releases.hashicorp.com/vault/$(VAULT_VERSION)/vault_$(VAULT_VERSION)_linux_amd64.zip && unzip vault_$(VAULT_VERSION)_linux_amd64.zip && chmod +x vault && mv vault /usr/local/bin/)
ifndef TEST_WINDOWS
	curl -LO https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl && chmod +x ./kubectl && mv kubectl /usr/local/bin/
	make setup-kind
endif
	docker pull $(IMAGE_TAG) || make e2e-container

.PHONY: setup-kind
setup-kind:
	# Download and install kind
	kind --version | grep -q $(KIND_VERSION) || (curl -L https://github.com/kubernetes-sigs/kind/releases/download/v$(KIND_VERSION)/kind-linux-amd64 --output kind && chmod +x kind && mv kind /usr/local/bin/)
	# Create kind cluster
	kind delete cluster || true
	kind create cluster --image kindest/node:v$(KUBERNETES_VERSION)

.PHONY: e2e-container
e2e-container:
	docker buildx rm container-builder || true
	docker buildx create --use --name=container-builder
ifdef TEST_WINDOWS
		docker buildx build --no-cache --build-arg LDFLAGS=$(LDFLAGS) -t $(IMAGE_TAG)-linux-amd64 -f docker/Dockerfile --platform="linux/amd64" --push .
		docker buildx build --no-cache --build-arg LDFLAGS=$(LDFLAGS) -t $(IMAGE_TAG)-windows-1809-amd64 -f docker/windows.Dockerfile --platform="windows/amd64" --push .
		docker manifest create $(IMAGE_TAG) $(IMAGE_TAG)-linux-amd64 $(IMAGE_TAG)-windows-1809-amd64
		docker manifest inspect $(IMAGE_TAG)
		docker manifest push --purge $(IMAGE_TAG)
else
		REGISTRY="e2e" $(MAKE) image
		kind load docker-image --name kind e2e/secrets-store-csi:$(IMAGE_VERSION)
endif

.PHONY: install-helm
install-helm:
	curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash

.PHONY: e2e-teardown
e2e-teardown:
	helm delete csi-secrets-store --namespace default

.PHONY: install-driver
install-driver:
ifdef TEST_WINDOWS
		helm install csi-secrets-store manifest_staging/charts/secrets-store-csi-driver --namespace default --wait --timeout=15m -v=5 --debug \
			--set windows.image.pullPolicy="IfNotPresent" \
			--set windows.image.repository=$(REGISTRY)/$(IMAGE_NAME) \
			--set windows.image.tag=$(IMAGE_VERSION) \
			--set windows.enabled=true \
			--set linux.enabled=false
else
		helm install csi-secrets-store manifest_staging/charts/secrets-store-csi-driver --namespace default --wait --timeout=15m -v=5 --debug \
			--set linux.image.pullPolicy="IfNotPresent" \
			--set linux.image.repository="e2e/secrets-store-csi" \
			--set linux.image.tag=$(IMAGE_VERSION) \
			--set linux.image.pullPolicy="IfNotPresent"
endif

.PHONY: e2e-azure
e2e-azure: install-driver
	bats -t test/bats/azure.bats

.PHONY: e2e-image
e2e-image:
	docker buildx build --no-cache --build-arg LDFLAGS=$(LDFLAGS) -t secrets-store-csi:e2e  -f docker/Dockerfile --platform="linux/amd64" --output "type=docker,push=false" .

.PHONY: e2e-vault
e2e-vault:  # e2e-image
	$(MAKE) -C test/e2e run PROVIDER=vault

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	# Generate the base CRD/RBAC
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=secretproviderclasses-role paths="./apis/..." paths="./controllers" output:crd:artifacts:config=config/crd/bases
	cp config/crd/bases/* manifest_staging/charts/secrets-store-csi-driver/templates
	cp config/crd/bases/* manifest_staging/deploy/

	# generate rbac-secretproviderclass
	$(KUSTOMIZE) build config/rbac -o manifest_staging/deploy/rbac-secretproviderclass.yaml
	cp config/rbac/role.yaml config/rbac/role_binding.yaml config/rbac/serviceaccount.yaml manifest_staging/charts/secrets-store-csi-driver/templates/
	@sed -i '1s/^/{{ if .Values.rbac.install }}\n/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role.yaml
	@sed -i '1s/^/{{ if .Values.rbac.install }}\n/gm; s/namespace: .*/namespace: {{ .Release.Namespace }}/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role_binding.yaml
	@sed -i '1s/^/{{ if .Values.rbac.install }}\n/gm; s/namespace: .*/namespace: {{ .Release.Namespace }}/gm; $$s/$$/\n{{ include "sscd.labels" . | indent 2 }}\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/serviceaccount.yaml

	# Generate secret syncing specific RBAC 
	$(CONTROLLER_GEN) rbac:roleName=secretprovidersyncing-role paths="./controllers/syncsecret" output:dir=config/rbac-syncsecret
	$(KUSTOMIZE) build config/rbac-syncsecret -o manifest_staging/deploy/rbac-secretprovidersyncing.yaml
	cp config/rbac-syncsecret/role.yaml manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret.yaml
	cp config/rbac-syncsecret/role_binding.yaml manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret_binding.yaml
	@sed -i '1s/^/{{ if .Values.syncSecret.enabled }}\n/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret.yaml
	@sed -i '1s/^/{{ if .Values.syncSecret.enabled }}\n/gm; s/namespace: .*/namespace: {{ .Release.Namespace }}/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret_binding.yaml

generate-protobuf:
	protoc -I . provider/v1alpha1/service.proto --go_out=plugins=grpc:.

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# find or download controller-gen
# download controller-gen if necessary
# using v0.4.0 by default generates v1 CRDs
controller-gen:
ifeq (, $(shell which controller-gen))
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.0
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# find or download kustomize
# download kustomize if necessary
kustomize:
ifeq (, $(shell which kustomize))
	go get sigs.k8s.io/kustomize@v3.6.1
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

release-manifest:
	$(MAKE) manifests
	@sed -i "s/version: .*/version: ${NEWVERSION}/" manifest_staging/charts/secrets-store-csi-driver/Chart.yaml
	@sed -i "s/appVersion: .*/appVersion: ${NEWVERSION}/" manifest_staging/charts/secrets-store-csi-driver/Chart.yaml
	@sed -i "s/tag: .*/tag: ${NEWVERSION}/" manifest_staging/charts/secrets-store-csi-driver/values.yaml
	@sed -i "s/image tag | .*/image tag | \`${NEWVERSION}\` |/" manifest_staging/charts/secrets-store-csi-driver/README.md

promote-staging-manifest:
	$(MAKE) release-manifest
	@rm -rf deploy
	@cp -r manifest_staging/deploy .
	@rm -rf charts/secrets-store-csi-driver
	@cp -r manifest_staging/charts/secrets-store-csi-driver ./charts
	@helm package ./charts/secrets-store-csi-driver -d ./charts/
	@helm repo index ./charts --url https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/master/charts
