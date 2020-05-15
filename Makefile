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
IMAGE_VERSION?=v0.0.10
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

all: build

test: test-style
	go test $(GO_FILES) -v
	go vet $(GO_FILES)
test-style: setup
	@echo "==> Running static validations and linters <=="
	golangci-lint run
sanity-test:
	go test -v ./test/sanity
build: setup
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags ${LDFLAGS} -o _output/secrets-store-csi ./cmd/secrets-store-csi-driver
build-windows: setup
	CGO_ENABLED=0 GOOS=windows go build -a -ldflags ${LDFLAGS} -o _output/secrets-store-csi.exe ./cmd/secrets-store-csi-driver
image:
	docker buildx build --no-cache --build-arg LDFLAGS=${LDFLAGS} -t $(IMAGE_TAG) -f Dockerfile --platform="linux/amd64" --output "type=docker,push=false" .
image-windows:
	docker buildx build --no-cache --build-arg LDFLAGS=${LDFLAGS} -t $(IMAGE_TAG) -f windows.Dockerfile --platform="windows/amd64" --output "type=docker,push=false" .
clean:
	-rm -rf _output
setup: clean
	@echo "Setup..."
	$Q go env

ifndef HAS_GOLANGCI
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin v1.19.1
endif

.PHONY: mod
mod:
	@go mod tidy

KIND_VERSION ?= 0.6.0
KUBERNETES_VERSION ?= 1.15.3
VAULT_VERSION ?= 1.2.2

# USED for windows CI tests
.PHONY: e2e-test
e2e-test: e2e-bootstrap
	make e2e-azure
	
.PHONY: e2e-bootstrap
e2e-bootstrap: install-helm
	apt-get update && apt-get install bats && apt-get install gettext-base -y
	# Download and install Vault
	curl -LO https://releases.hashicorp.com/vault/${VAULT_VERSION}/vault_${VAULT_VERSION}_linux_amd64.zip && unzip vault_${VAULT_VERSION}_linux_amd64.zip && chmod +x vault && mv vault /usr/local/bin/
ifndef TEST_WINDOWS
	curl -LO https://storage.googleapis.com/kubernetes-release/release/v${KUBERNETES_VERSION}/bin/linux/amd64/kubectl && chmod +x ./kubectl && mv kubectl /usr/local/bin/
	make setup-kind
endif
	docker pull $(IMAGE_TAG) || make e2e-container

.PHONY: setup-kind
setup-kind:
	# Download and install kind
	curl -L https://github.com/kubernetes-sigs/kind/releases/download/v${KIND_VERSION}/kind-linux-amd64 --output kind && chmod +x kind && mv kind /usr/local/bin/
	# Create kind cluster
	kind create cluster --config kind-config.yaml --image kindest/node:v${KUBERNETES_VERSION}

.PHONY: e2e-container
e2e-container:
	docker buildx rm container-builder || true
	docker buildx create --use --name=container-builder
ifdef TEST_WINDOWS
		az acr login --name $(REGISTRY_NAME)
		docker buildx build --no-cache --build-arg LDFLAGS=${LDFLAGS} -t $(IMAGE_TAG)-linux-amd64 -f Dockerfile --platform="linux/amd64" --push .
		docker buildx build --no-cache --build-arg LDFLAGS=${LDFLAGS} -t $(IMAGE_TAG)-windows-1809-amd64 -f windows.Dockerfile --platform="windows/amd64" --push .
		docker manifest create $(IMAGE_TAG) $(IMAGE_TAG)-linux-amd64 $(IMAGE_TAG)-windows-1809-amd64
		docker manifest inspect $(IMAGE_TAG)
		docker manifest push --purge $(IMAGE_TAG)
else
		REGISTRY="e2e" make image
		kind load docker-image --name kind e2e/secrets-store-csi:${IMAGE_VERSION}
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
		helm install csi-secrets-store charts/secrets-store-csi-driver --namespace default --wait --timeout=15m -v=5 --debug \
			--set windows.image.pullPolicy="IfNotPresent" \
			--set windows.image.repository=$(REGISTRY)/$(IMAGE_NAME) \
			--set windows.image.tag=$(IMAGE_VERSION) \
			--set windows.enabled=true \
			--set linux.enabled=false
else
		helm install csi-secrets-store charts/secrets-store-csi-driver --namespace default --wait --timeout=15m -v=5 --debug \
			--set linux.image.pullPolicy="IfNotPresent" \
			--set linux.image.repository="e2e/secrets-store-csi" \
			--set linux.image.tag=$(IMAGE_VERSION) \
			--set linux.image.pullPolicy="IfNotPresent"
endif

.PHONY: e2e-azure
e2e-azure: install-driver
	bats -t test/bats/azure.bats

.PHONY: e2e-vault
e2e-vault: install-driver
	bats -t test/bats/vault.bats
