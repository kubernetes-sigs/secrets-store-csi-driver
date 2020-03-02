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

REGISTRY_NAME?=docker.io/deislabs
IMAGE_NAME=secrets-store-csi
IMAGE_VERSION?=v0.0.7
IMAGE_TAG=$(REGISTRY_NAME)/$(IMAGE_NAME):$(IMAGE_VERSION)
IMAGE_TAG_LATEST=$(REGISTRY_NAME)/$(IMAGE_NAME):latest
LDFLAGS?='-X sigs.k8s.io/secrets-store-csi-driver/pkg/secrets-store.vendorVersion=$(IMAGE_VERSION) -extldflags "-static"'
GO_FILES=$(shell go list ./... | grep -v /test/sanity)

.PHONY: all build image clean test-style

GO111MODULE ?= on
export GO111MODULE

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
image: build
	docker build --no-cache -t $(IMAGE_TAG) -f Dockerfile .
docker-login:
	echo $(DOCKER_PASSWORD) | docker login -u $(DOCKER_USERNAME) --password-stdin
ci-deploy: image docker-login
	docker push $(IMAGE_TAG)
	docker tag $(IMAGE_TAG) $(IMAGE_TAG_LATEST)
	docker push $(IMAGE_TAG_LATEST)
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

KIND_VERSION ?= 0.5.1
KUBERNETES_VERSION ?= 1.15.3
VAULT_VERSION ?= 1.2.2
E2E_IMAGE_VERSION = v0.0.8-e2e-$$(git rev-parse --short HEAD)

.PHONY: e2e-bootstrap
e2e-bootstrap:
	# Download and install kubectl
	curl -LO https://storage.googleapis.com/kubernetes-release/release/v${KUBERNETES_VERSION}/bin/linux/amd64/kubectl && chmod +x ./kubectl && mv kubectl /usr/local/bin/
	# Download and install kind
	curl -L https://github.com/kubernetes-sigs/kind/releases/download/v${KIND_VERSION}/kind-linux-amd64 --output kind && chmod +x kind && mv kind /usr/local/bin/
	# Download and install Helm
	curl https://raw.githubusercontent.com/helm/helm/master/scripts/get | bash
	# Download and install Vault
	curl -LO https://releases.hashicorp.com/vault/${VAULT_VERSION}/vault_${VAULT_VERSION}_linux_amd64.zip && unzip vault_${VAULT_VERSION}_linux_amd64.zip && chmod +x vault && mv vault /usr/local/bin/
	# Create kind cluster
	kind create cluster --config kind-config.yaml --image kindest/node:v${KUBERNETES_VERSION}
	# Build image
	REGISTRY_NAME="e2e" IMAGE_VERSION=${E2E_IMAGE_VERSION} make image
	# Load image into kind cluster
	kind load docker-image --name kind e2e/secrets-store-csi:${E2E_IMAGE_VERSION}
	# Set up tiller
	kubectl --namespace kube-system --output yaml create serviceaccount tiller --dry-run | kubectl --kubeconfig $$(kind get kubeconfig-path)  apply -f -
	kubectl create --output yaml clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller --dry-run | kubectl --kubeconfig $$(kind get kubeconfig-path) apply -f -
	helm init --service-account tiller --upgrade --wait --kubeconfig $$(kind get kubeconfig-path)

.PHONY: e2e-azure
e2e-azure:
	bats -t test/bats/azure.bats

.PHONY: e2e-vault
e2e-vault:
	bats -t test/bats/vault.bats
