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

REGISTRY_NAME=ritazh
IMAGE_NAME=secrets-store-csi
IMAGE_VERSION=v0.0.4
IMAGE_TAG=$(REGISTRY_NAME)/$(IMAGE_NAME):$(IMAGE_VERSION)
IMAGE_TAG_LATEST=$(REGISTRY_NAME)/$(IMAGE_NAME):latest
REV=$(shell git describe --long --tags --dirty)
LDFLAGS?='-X github.com/deislabs/secrets-store-csi-driver/pkg/secrets-store.vendorVersion=$(IMAGE_VERSION) -extldflags "-static"'

.PHONY: all build image clean test-style

GO111MODULE ?= on
export GO111MODULE

HAS_GOLANGCI := $(shell command -v golangci-lint;)

all: build

test: test-style
	go test github.com/deislabs/secrets-store-csi-driver/pkg/... -cover
	go vet github.com/deislabs/secrets-store-csi-driver/pkg/...
test-style: setup
	@echo "==> Running static validations and linters <=="
	golangci-lint run
build: setup
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags ${LDFLAGS} -o _output/secrets-store-csi ./pkg/secrets-store-csi-driver
image: build
	docker build --no-cache -t $(IMAGE_TAG) -f ./pkg/secrets-store-csi-driver/Dockerfile .
push: image
	docker push $(IMAGE_TAG)
push-latest: image
	docker push $(IMAGE_TAG)
	docker tag $(IMAGE_TAG) $(IMAGE_TAG_LATEST)
	docker push $(IMAGE_TAG_LATEST)
clean:
	-rm -rf _output
setup: clean
	@echo "Setup..."
	$Q go env

ifndef HAS_GOLANGCI
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin
endif

.PHONY: unit-test
unit-test:
	go test -v -race ./pkg/...

.PHONY: mod
mod:
	@go mod tidy