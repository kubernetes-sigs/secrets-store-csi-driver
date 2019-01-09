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
IMAGE_NAME=keyvault-csi
IMAGE_VERSION=v0.0.3
IMAGE_TAG=$(REGISTRY_NAME)/$(IMAGE_NAME):$(IMAGE_VERSION)
IMAGE_TAG_LATEST=$(REGISTRY_NAME)/$(IMAGE_NAME):latest
REV=$(shell git describe --long --tags --dirty)

.PHONY: all keyvault-csi image clean deps test-style

HAS_DEP := $(shell command -v dep;)
HAS_GOLANGCI := $(shell command -v golangci-lint;)

all: keyvault-csi

test: test-style
	go test github.com/ritazh/keyvault-csi-driver/pkg/... -cover
	go vet github.com/ritazh/keyvault-csi-driver/pkg/...
test-style: setup
	@echo "==> Running static validations and linters <=="
	golangci-lint run
keyvault-csi: deps
	if [ ! -d ./vendor ]; then dep ensure -vendor-only; fi
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-X github.com/ritazh/keyvault-csi-driver/pkg/keyvault.vendorVersion=$(IMAGE_VERSION) -extldflags "-static"' -o _output/keyvaultcsi ./pkg/keyvaultcsidriver
image: keyvault-csi
	docker build --no-cache -t $(IMAGE_TAG) -f ./pkg/keyvaultcsidriver/Dockerfile .
push: image
	docker push $(IMAGE_TAG)
push-latest: image
	docker push $(IMAGE_TAG)
	docker tag $(IMAGE_TAG) $(IMAGE_TAG_LATEST)
	docker push $(IMAGE_TAG_LATEST)
clean:
	go clean -r -x
	-rm -rf _output
setup: clean
	@echo "Setup..."
ifndef HAS_DEP
	go get -u github.com/golang/dep/cmd/dep
endif
ifndef HAS_GOLANGCI
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin
endif
deps: setup
	@echo "Ensuring Dependencies..."
	$Q go env
	$Q dep ensure
