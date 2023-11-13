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

GOPATH  := $(shell go env GOPATH)
GOARCH  := $(shell go env GOARCH)
GOOS    := $(shell go env GOOS)
GOPROXY := $(shell go env GOPROXY)

ORG_PATH=sigs.k8s.io
PROJECT_NAME := secrets-store-csi-driver
BUILD_COMMIT := $(shell git rev-parse --short HEAD)
REPO_PATH="$(ORG_PATH)/$(PROJECT_NAME)"

REGISTRY ?= gcr.io/k8s-staging-csi-secrets-store
IMAGE_NAME ?= driver
CRD_IMAGE_NAME ?= driver-crds
E2E_PROVIDER_IMAGE_NAME ?= e2e-provider

# Release version is the current supported release for the driver
# Update this version when the helm chart is being updated for release
RELEASE_VERSION := v1.3.4
IMAGE_VERSION ?= v1.3.4

# Use a custom version for E2E tests if we are testing in CI
ifdef CI
override IMAGE_VERSION := v1.3.0-e2e-$(BUILD_COMMIT)
endif

IMAGE_TAG=$(REGISTRY)/$(IMAGE_NAME):$(IMAGE_VERSION)
CRD_IMAGE_TAG=$(REGISTRY)/$(CRD_IMAGE_NAME):$(IMAGE_VERSION)
E2E_PROVIDER_IMAGE_TAG=$(REGISTRY)/$(E2E_PROVIDER_IMAGE_NAME):$(IMAGE_VERSION)

# build variables
BUILD_TIMESTAMP := $$(date +%Y-%m-%d-%H:%M)
BUILD_TIME_VAR := $(REPO_PATH)/pkg/version.BuildTime
BUILD_VERSION_VAR := $(REPO_PATH)/pkg/version.BuildVersion
VCS_VAR := $(REPO_PATH)/pkg/version.Vcs
LDFLAGS ?= "-X $(BUILD_TIME_VAR)=$(BUILD_TIMESTAMP) -X $(BUILD_VERSION_VAR)=$(IMAGE_VERSION) -X $(VCS_VAR)=$(BUILD_COMMIT)"

GO_FILES=$(shell go list ./... | grep -v /test/sanity)
TOOLS_MOD_DIR := ./hack/tools
TOOLS_DIR := $(abspath ./hack/tools)
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin

# we use go modules to manage dependencies
GO111MODULE = on
# for using docker buildx and docker manifest command
DOCKER_CLI_EXPERIMENTAL = enabled
export GOPATH GOBIN GO111MODULE DOCKER_CLI_EXPERIMENTAL

# Generate all combination of all OS, ARCH, and OSVERSIONS for iteration
ALL_OS = linux windows
ALL_ARCH.linux = amd64 arm64
ALL_OS_ARCH.linux = $(foreach arch, ${ALL_ARCH.linux}, linux-$(arch))
ALL_ARCH.windows = amd64
ALL_OSVERSIONS.windows := 1809 ltsc2022
ALL_OS_ARCH.windows = $(foreach arch, $(ALL_ARCH.windows), $(foreach osversion, ${ALL_OSVERSIONS.windows}, windows-${osversion}-${arch}))
ALL_OS_ARCH = $(foreach os, $(ALL_OS), ${ALL_OS_ARCH.${os}})

# The current context of image building
# The architecture of the image
ARCH ?= amd64
# OS Version for the Windows images: 1809, ltsc2022
OSVERSION ?= 1809
# Output type of docker buildx build
OUTPUT_TYPE ?= registry
BUILDX_BUILDER_NAME ?= img-builder
QEMU_VERSION ?= 5.2.0-2
# pinning buildkit version to v0.10.6 as v0.11.0 is injecting sbom/prov to manifest
# causing the manifest push to fail
BUILDKIT_VERSION ?= v0.10.6

# Binaries
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/controller-gen
KUSTOMIZE := $(TOOLS_BIN_DIR)/kustomize
PROTOC := $(TOOLS_DIR)/bin/protoc
PROTOC_GEN_GO := $(TOOLS_DIR)/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(TOOLS_DIR)/bin/protoc-gen-go-grpc
TRIVY := trivy
HELM := helm
BATS := bats
AZURE_CLI := az
KIND := kind
KUBECTL := kubectl
ENVSUBST := envsubst
EKSCTL := eksctl
YQ := yq

# Test variables
KIND_VERSION ?= 0.18.0
KUBERNETES_VERSION ?= 1.24.0
KUBECTL_VERSION ?= 1.25.3
BATS_VERSION ?= 1.4.1
TRIVY_VERSION ?= 0.39.1
PROTOC_VERSION ?= 3.20.1
SHELLCHECK_VER ?= v0.8.0
YQ_VERSION ?= v4.11.2

# For aws integration tests
BUILD_TIMESTAMP_W_SEC := $(shell date +%Y-%m-%d-%H-%M-%S)
EKS_CLUSTER_NAME := integ-cluster-$(BUILD_TIMESTAMP_W_SEC)
AWS_REGION := us-west-2

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:crdVersions=v1"

## --------------------------------------
## Validate golang version
## --------------------------------------
GO_MAJOR_VERSION = $(shell go version | cut -c 14- | cut -d' ' -f1 | cut -d'.' -f1)
GO_MINOR_VERSION = $(shell go version | cut -c 14- | cut -d' ' -f1 | cut -d'.' -f2)
MINIMUM_SUPPORTED_GO_MAJOR_VERSION = 1
MINIMUM_SUPPORTED_GO_MINOR_VERSION = 16
GO_VERSION_VALIDATION_ERR_MSG = Golang version is not supported, please update to at least $(MINIMUM_SUPPORTED_GO_MAJOR_VERSION).$(MINIMUM_SUPPORTED_GO_MINOR_VERSION)

.PHONY: validate-go
validate-go: ## Validates the installed version of go.
	@if [ $(GO_MAJOR_VERSION) -gt $(MINIMUM_SUPPORTED_GO_MAJOR_VERSION) ]; then \
		exit 0 ;\
	elif [ $(GO_MAJOR_VERSION) -lt $(MINIMUM_SUPPORTED_GO_MAJOR_VERSION) ]; then \
		echo '$(GO_VERSION_VALIDATION_ERR_MSG)';\
		exit 1; \
	elif [ $(GO_MINOR_VERSION) -lt $(MINIMUM_SUPPORTED_GO_MINOR_VERSION) ] ; then \
		echo '$(GO_VERSION_VALIDATION_ERR_MSG)';\
		exit 1; \
	fi

## --------------------------------------
## Testing
## --------------------------------------

.PHONY: test
test: go-test

.PHONY: go-test # Run unit tests
go-test:
	go test -count=1 $(GO_FILES) -v -coverprofile cover.out
	cd test/e2eprovider && go test ./... -tags e2e -count=1 -v

# skipping Controller tests as this driver only implements Node and Identity service.
.PHONY: sanity-test # Run CSI sanity tests for the driver
sanity-test:
	go test -v ./test/sanity -ginkgo.skip=Controller\|should.work\|NodeStageVolume

.PHONY: image-scan
image-scan: $(TRIVY)
	# show all vulnerabilities
	$(TRIVY) image --severity MEDIUM,HIGH,CRITICAL $(IMAGE_TAG)
	$(TRIVY) image --severity MEDIUM,HIGH,CRITICAL $(CRD_IMAGE_TAG)
	# show vulnerabilities that have been fixed
	$(TRIVY) image --exit-code 1 --ignore-unfixed --severity MEDIUM,HIGH,CRITICAL $(IMAGE_TAG)
	$(TRIVY) image --exit-code 1 --ignore-unfixed --severity MEDIUM,HIGH,CRITICAL $(CRD_IMAGE_TAG)

## --------------------------------------
## Tooling Binaries
## --------------------------------------

$(CONTROLLER_GEN): $(TOOLS_MOD_DIR)/go.mod $(TOOLS_MOD_DIR)/go.sum $(TOOLS_MOD_DIR)/tools.go ## Build controller-gen from tools folder.
	cd $(TOOLS_MOD_DIR) && \
		GOPROXY=$(GOPROXY) go build -tags=tools -o $(TOOLS_BIN_DIR)/controller-gen sigs.k8s.io/controller-tools/cmd/controller-gen

$(GOLANGCI_LINT): ## Build golangci-lint from tools folder.
	cd $(TOOLS_MOD_DIR) && \
		GOPROXY=$(GOPROXY) go build -o $(TOOLS_BIN_DIR)/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint

$(KUSTOMIZE): ## Build kustomize from tools folder.
	cd $(TOOLS_MOD_DIR) && \
		GOPROXY=$(GOPROXY) go build -tags=tools -o $(TOOLS_BIN_DIR)/kustomize sigs.k8s.io/kustomize/kustomize/v4

$(PROTOC_GEN_GO): ## Build protoc-gen-go from tools folder.
	cd $(TOOLS_MOD_DIR) && \
		GOPROXY=$(GOPROXY) go build -tags=tools -o $(TOOLS_BIN_DIR)/protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go

$(PROTOC_GEN_GO_GRPC): ## Build protoc-gen-go-grpc from tools folder.
	cd $(TOOLS_MOD_DIR) && \
		GOPROXY=$(GOPROXY) go build -tags=tools -o $(TOOLS_BIN_DIR)/protoc-gen-go-grpc google.golang.org/grpc/cmd/protoc-gen-go-grpc

## --------------------------------------
## Testing Binaries
## --------------------------------------

$(HELM): ## Install helm3 if not present
	helm version --short | grep -q v3 || (curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash)

$(KIND): ## Download and install kind
	kind --version | grep -q $(KIND_VERSION) || (curl -L https://github.com/kubernetes-sigs/kind/releases/download/v$(KIND_VERSION)/kind-linux-amd64 --output kind && chmod +x kind && mv kind /usr/local/bin/)

$(AZURE_CLI): ## Download and install azure cli
	curl -sL https://aka.ms/InstallAzureCLIDeb | bash

$(EKSCTL): ## Download and install eksctl
	curl -sSLO  https://github.com/weaveworks/eksctl/releases/latest/download/eksctl_Linux_amd64.tar.gz && tar -zxvf eksctl_Linux_amd64.tar.gz && chmod +x eksctl && mv eksctl /usr/local/bin/

$(KUBECTL): ## Install kubectl
	curl -LO https://dl.k8s.io/release/v$(KUBECTL_VERSION)/bin/linux/amd64/kubectl && chmod +x ./kubectl && mv kubectl /usr/local/bin/

$(TRIVY): ## Install trivy for image vulnerability scan
	trivy -v | grep -q $(TRIVY_VERSION) || (curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin v$(TRIVY_VERSION))

$(BATS): ## Install bats for running the tests
	bats --version | grep -q $(BATS_VERSION) || (curl -sSLO https://github.com/bats-core/bats-core/archive/v${BATS_VERSION}.tar.gz && tar -zxvf v${BATS_VERSION}.tar.gz && bash bats-core-${BATS_VERSION}/install.sh /usr/local)

$(ENVSUBST): ## Install envsubst for running the tests
	envsubst -V || (apt-get -o Acquire::Retries=30 update && apt-get -o Acquire::Retries=30 install gettext-base -y)

$(PROTOC): ## Install protoc
	curl -sSLO https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip && unzip protoc-${PROTOC_VERSION}-linux-x86_64.zip bin/protoc -d $(TOOLS_DIR) && rm protoc-${PROTOC_VERSION}-linux-x86_64.zip

$(YQ): ## Install yq for running the tests
	curl -LO https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_linux_amd64 && chmod +x ./yq_linux_amd64 && mv yq_linux_amd64 /usr/local/bin/yq

SHELLCHECK := $(TOOLS_BIN_DIR)/shellcheck-$(SHELLCHECK_VER)
$(SHELLCHECK): OS := $(shell uname | tr '[:upper:]' '[:lower:]')
$(SHELLCHECK): ARCH := $(shell uname -m)
$(SHELLCHECK):
	mkdir -p $(TOOLS_BIN_DIR)
	rm -rf "$(SHELLCHECK)*"
	curl -sfOL "https://github.com/koalaman/shellcheck/releases/download/$(SHELLCHECK_VER)/shellcheck-$(SHELLCHECK_VER).$(OS).$(ARCH).tar.xz"
	tar xf shellcheck-$(SHELLCHECK_VER).$(OS).$(ARCH).tar.xz
	cp "shellcheck-$(SHELLCHECK_VER)/shellcheck" "$(SHELLCHECK)"
	ln -sf "$(SHELLCHECK)" "$(TOOLS_BIN_DIR)/shellcheck"
	chmod +x "$(TOOLS_BIN_DIR)/shellcheck" "$(SHELLCHECK)"
	rm -rf shellcheck*

## --------------------------------------
## Linting
## --------------------------------------
.PHONY: test-style
test-style: lint lint-charts shellcheck

.PHONY: lint
lint: $(GOLANGCI_LINT)
	# Setting timeout to 5m as default is 1m
	$(GOLANGCI_LINT) run --timeout=5m -v
	cd test/e2eprovider && $(GOLANGCI_LINT) run --build-tags e2e --timeout=5m -v

lint-full: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run -v --fast=false

lint-charts: $(HELM) # Run helm lint tests
	helm lint charts/secrets-store-csi-driver
	helm lint manifest_staging/charts/secrets-store-csi-driver

.PHONY: shellcheck
shellcheck: $(SHELLCHECK)
	find . -name '*.sh' -not -path './third_party/*'  | xargs $(SHELLCHECK)

## --------------------------------------
## Builds
## --------------------------------------
.PHONY: build
build:
	GOPROXY=$(GOPROXY) CGO_ENABLED=0 GOOS=linux go build -a -ldflags $(LDFLAGS) -o _output/secrets-store-csi ./cmd/secrets-store-csi-driver

.PHONY: build-e2e-provider
build-e2e-provider:
	cd test/e2eprovider && GOPROXY=$(GOPROXY) CGO_ENABLED=0 GOOS=linux go build -a -tags "e2e" -o e2e-provider

.PHONY: build-windows
build-windows:
	GOPROXY=$(GOPROXY) CGO_ENABLED=0 GOOS=windows go build -a -ldflags $(LDFLAGS) -o _output/secrets-store-csi.exe ./cmd/secrets-store-csi-driver

.PHONY: build-darwin
build-darwin:
	GOPROXY=$(GOPROXY) CGO_ENABLED=0 GOOS=darwin go build -a -ldflags $(LDFLAGS) -o _output/secrets-store-csi ./cmd/secrets-store-csi-driver

.PHONY: clean-crds
clean-crds:
	rm -rf _output/crds/*

.PHONY: build-crds
build-crds: clean-crds
	mkdir -p _output/crds
ifdef CI
	cp -R manifest_staging/charts/secrets-store-csi-driver/crds/ _output/crds/
else
	cp -R charts/secrets-store-csi-driver/crds/ _output/crds/
endif

.PHONY: e2e-provider-container
e2e-provider-container:
	docker buildx build --no-cache -t $(E2E_PROVIDER_IMAGE_TAG) -f test/e2eprovider/Dockerfile --progress=plain .

.PHONY: container
container: crd-container
	docker buildx build --no-cache --build-arg IMAGE_VERSION=$(IMAGE_VERSION) -t $(IMAGE_TAG) -f docker/Dockerfile --progress=plain .

.PHONY: crd-container
crd-container: build-crds
	docker buildx build --no-cache -t $(CRD_IMAGE_TAG) -f docker/crd.Dockerfile --progress=plain _output/crds/

.PHONY: crd-container-linux
crd-container-linux: build-crds docker-buildx-builder
	docker buildx build --no-cache --output=type=$(OUTPUT_TYPE) --platform="linux/$(ARCH)" \
		-t $(CRD_IMAGE_TAG)-linux-$(ARCH) -f docker/crd.Dockerfile _output/crds/

.PHONY: container-linux
container-linux: docker-buildx-builder
	docker buildx build --no-cache --build-arg IMAGE_VERSION=$(IMAGE_VERSION) --output=type=$(OUTPUT_TYPE) --platform="linux/$(ARCH)" \
 		-t $(IMAGE_TAG)-linux-$(ARCH) -f docker/Dockerfile .

.PHONY: container-windows
container-windows: docker-buildx-builder
	docker buildx build --no-cache --build-arg IMAGE_VERSION=$(IMAGE_VERSION) --output=type=$(OUTPUT_TYPE) --platform="windows/$(ARCH)" \
		--build-arg BASEIMAGE=mcr.microsoft.com/windows/nanoserver:$(OSVERSION) \
		--build-arg BASEIMAGE_CORE=gcr.io/k8s-staging-e2e-test-images/windows-servercore-cache:1.0-linux-amd64-$(OSVERSION) \
 		-t $(IMAGE_TAG)-windows-$(OSVERSION)-$(ARCH) -f docker/windows.Dockerfile .

.PHONY: docker-buildx-builder
docker-buildx-builder:
	@if ! docker buildx ls | grep $(BUILDX_BUILDER_NAME); then \
		docker run --rm --privileged multiarch/qemu-user-static:$(QEMU_VERSION) --reset -p yes; \
		docker buildx create --driver-opt image=moby/buildkit:$(BUILDKIT_VERSION) --name $(BUILDX_BUILDER_NAME) --use; \
		docker buildx inspect $(BUILDX_BUILDER_NAME) --bootstrap; \
	fi

.PHONY: container-all
container-all: docker-buildx-builder
	for arch in $(ALL_ARCH.linux); do \
		ARCH=$${arch} $(MAKE) container-linux; \
		ARCH=$${arch} $(MAKE) crd-container-linux; \
	done
	for osversion in $(ALL_OSVERSIONS.windows); do \
  		OSVERSION=$${osversion} $(MAKE) container-windows; \
  	done

.PHONY: push-manifest
push-manifest:
	docker manifest create --amend $(IMAGE_TAG) $(foreach osarch, $(ALL_OS_ARCH), $(IMAGE_TAG)-${osarch})
	docker manifest create --amend $(CRD_IMAGE_TAG) $(foreach osarch, $(ALL_OS_ARCH.linux), $(CRD_IMAGE_TAG)-${osarch})
	# add "os.version" field to windows images (based on https://github.com/kubernetes/kubernetes/blob/master/build/pause/Makefile)
	set -x; \
	registry_prefix=$(shell (echo ${REGISTRY} | grep -Eq ".*[\/\.].*") && echo "" || echo "docker.io/"); \
	manifest_image_folder=`echo "$${registry_prefix}${IMAGE_TAG}" | sed "s|/|_|g" | sed "s/:/-/"`; \
	for arch in $(ALL_ARCH.windows); do \
		for osversion in $(ALL_OSVERSIONS.windows); do \
			BASEIMAGE=mcr.microsoft.com/windows/nanoserver:$${osversion}; \
			full_version=`docker manifest inspect $${BASEIMAGE} | jq -r '.manifests[0].platform["os.version"]'`; \
			sed -i -r "s/(\"os\"\:\"windows\")/\0,\"os.version\":\"$${full_version}\"/" "${HOME}/.docker/manifests/$${manifest_image_folder}/$${manifest_image_folder}-windows-$${osversion}-$${arch}"; \
		done; \
	done
	docker manifest push --purge $(IMAGE_TAG)
	docker manifest inspect $(IMAGE_TAG)
	docker manifest push --purge $(CRD_IMAGE_TAG)
	docker manifest inspect $(CRD_IMAGE_TAG)

## --------------------------------------
## E2E Testing
## --------------------------------------
.PHONY: e2e-bootstrap
e2e-bootstrap: $(HELM) $(BATS) $(KIND) $(KUBECTL) $(ENVSUBST) $(YQ) #setup all required binaries and kind cluster for testing
ifndef TEST_WINDOWS
	$(MAKE) setup-kind
endif
	docker pull $(IMAGE_TAG) || $(MAKE) e2e-container

.PHONY: setup-kind
setup-kind: $(KIND)
	# (Re)create kind cluster
	if [ $$(kind get clusters) ]; then kind delete cluster; fi
	kind create cluster --image kindest/node:v$(KUBERNETES_VERSION)

.PHONY: setup-eks-cluster
setup-eks-cluster: $(HELM) $(EKSCTL) $(BATS) $(ENVSUBST) $(YQ)
	bash test/scripts/initialize_eks_cluster.bash $(EKS_CLUSTER_NAME) $(IMAGE_VERSION)

.PHONY: e2e-container
e2e-container:
ifdef TEST_WINDOWS
	$(MAKE) container-all push-manifest
else
	$(MAKE) container
	kind load docker-image --name kind $(IMAGE_TAG) $(CRD_IMAGE_TAG)
endif

.PHONY: e2e-mock-provider-container
e2e-mock-provider-container:
	$(MAKE) e2e-provider-container
	kind load docker-image --name kind $(E2E_PROVIDER_IMAGE_TAG)

.PHONY: e2e-test
e2e-test: e2e-bootstrap e2e-helm-deploy # run test for windows
	$(MAKE) e2e-azure

.PHONY: e2e-teardown
e2e-teardown: $(HELM)
	helm delete csi-secrets-store --namespace kube-system

.PHONY: e2e-provider-deploy
e2e-provider-deploy:
	yq e 'select(.kind == "DaemonSet").spec.template.spec.containers[0].image = "$(E2E_PROVIDER_IMAGE_TAG)"' 'test/e2eprovider/e2e-provider-installer.yaml' | kubectl apply -n kube-system -f -

.PHONY: e2e-deploy-manifest
e2e-deploy-manifest:
	kubectl apply -f manifest_staging/deploy/csidriver.yaml
	kubectl apply -f manifest_staging/deploy/rbac-secretproviderclass.yaml
	kubectl apply -f manifest_staging/deploy/rbac-secretproviderrotation.yaml
	kubectl apply -f manifest_staging/deploy/rbac-secretprovidersyncing.yaml
	kubectl apply -f manifest_staging/deploy/rbac-secretprovidertokenrequest.yaml
	kubectl apply -f manifest_staging/deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml
	kubectl apply -f manifest_staging/deploy/secrets-store.csi.x-k8s.io_secretproviderclasspodstatuses.yaml
	kubectl apply -f manifest_staging/deploy/role-secretproviderclasses-admin.yaml
	kubectl apply -f manifest_staging/deploy/role-secretproviderclasses-viewer.yaml
	kubectl apply -f manifest_staging/deploy/role-secretproviderclasspodstatuses-viewer.yaml

	yq e '(.spec.template.spec.containers[1].image = "$(IMAGE_TAG)") | (.spec.template.spec.containers[1].args as $$x | $$x += "--enable-secret-rotation=true" | $$x[-1] style="double") | (.spec.template.spec.containers[1].args as $$x | $$x += "--rotation-poll-interval=30s" | $$x[-1] style="double")' 'manifest_staging/deploy/secrets-store-csi-driver.yaml' | kubectl apply -f -

	yq e '(.spec.template.spec.containers[1].args as $$x | $$x += "--enable-secret-rotation=true" | $$x[-1] style="double") | (.spec.template.spec.containers[1].args as $$x | $$x += "--rotation-poll-interval=30s" | $$x[-1] style="double")' 'manifest_staging/deploy/secrets-store-csi-driver-windows.yaml' | kubectl apply -f -

.PHONY: e2e-helm-deploy
e2e-helm-deploy:
	helm install csi-secrets-store manifest_staging/charts/secrets-store-csi-driver --namespace kube-system --wait --timeout=5m -v=5 --debug \
		--set linux.image.pullPolicy="IfNotPresent" \
		--set windows.image.pullPolicy="IfNotPresent" \
		--set linux.image.repository=$(REGISTRY)/$(IMAGE_NAME) \
		--set linux.image.tag=$(IMAGE_VERSION) \
		--set windows.image.repository=$(REGISTRY)/$(IMAGE_NAME) \
		--set windows.image.tag=$(IMAGE_VERSION) \
		--set linux.crds.image.repository=$(REGISTRY)/$(CRD_IMAGE_NAME) \
		--set linux.crds.image.tag=$(IMAGE_VERSION) \
		--set linux.crds.annotations."myAnnotation"=test \
		--set windows.enabled=true \
		--set linux.enabled=true \
		--set syncSecret.enabled=true \
		--set enableSecretRotation=true \
		--set rotationPollInterval=30s \
		--set tokenRequests[0].audience="aud1" \
		--set tokenRequests[1].audience="aud2"

.PHONY: e2e-helm-upgrade
e2e-helm-upgrade:
	helm upgrade csi-secrets-store manifest_staging/charts/secrets-store-csi-driver --namespace kube-system --reuse-values --timeout=5m -v=5 --debug \
		--set linux.image.repository=$(REGISTRY)/$(IMAGE_NAME) \
		--set linux.image.tag=$(IMAGE_VERSION) \
		--set windows.image.repository=$(REGISTRY)/$(IMAGE_NAME) \
		--set windows.image.tag=$(IMAGE_VERSION) \
		--set linux.crds.image.repository=$(REGISTRY)/$(CRD_IMAGE_NAME) \
		--set linux.crds.image.tag=$(IMAGE_VERSION) \
		--set linux.crds.annotations."myAnnotation"=test

.PHONY: e2e-helm-deploy-release # test helm package for the release
e2e-helm-deploy-release:
	set -x; \
	helm install csi-secrets-store charts/secrets-store-csi-driver --namespace kube-system --wait --timeout=5m -v=5 --debug \
		--set linux.image.pullPolicy="IfNotPresent" \
		--set windows.image.pullPolicy="IfNotPresent" \
		--set windows.enabled=true \
		--set linux.enabled=true \
		--set syncSecret.enabled=true \
		--set enableSecretRotation=true \
		--set rotationPollInterval=30s

.PHONY: e2e-kind-cleanup
e2e-kind-cleanup:
	kind delete cluster --name kind

.PHONY: e2e-eks-cleanup
e2e-eks-cleanup:
	eksctl delete cluster --name $(EKS_CLUSTER_NAME) --region $(AWS_REGION)

.PHONY: e2e-provider
e2e-provider:
	bats -t -T test/bats/e2e-provider.bats

.PHONY: e2e-azure
e2e-azure: $(AZURE_CLI)
	bats -t test/bats/azure.bats

.PHONY: e2e-vault
e2e-vault:
	bats -t test/bats/vault.bats

.PHONY: e2e-akeyless
e2e-akeyless:
	bats -t test/bats/akeyless.bats

.PHONY: e2e-gcp
e2e-gcp:
	bats -t test/bats/gcp.bats

.PHONY: e2e-aws
e2e-aws:
	bats -t test/bats/aws.bats

## --------------------------------------
## Generate
## --------------------------------------
# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: $(CONTROLLER_GEN) $(KUSTOMIZE)
	# Generate the base CRD/RBAC
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=secretproviderclasses-role object:headerFile=./hack/boilerplate.go.txt paths="./apis/..." \
		 paths="./apis/..." paths="./controllers" output:crd:artifacts:config=config/crd/bases
	cp config/crd/bases/* manifest_staging/charts/secrets-store-csi-driver/crds
	cp config/crd/bases/* manifest_staging/deploy/

	# generate rbac-secretproviderclass
	$(KUSTOMIZE) build config/rbac -o manifest_staging/deploy/rbac-secretproviderclass.yaml
	cp config/rbac/role.yaml config/rbac/role_binding.yaml config/rbac/serviceaccount.yaml manifest_staging/charts/secrets-store-csi-driver/templates/
	@sed -i '1s/^/{{ if .Values.rbac.install }}\n/gm; $$s/$$/\n{{- if and .Values.rbac.pspEnabled \(\.Capabilities.APIVersions.Has \"policy\/v1beta1\/PodSecurityPolicy\"\) }}\n- apiGroups:\n  - policy\n  resources:\n  - podsecuritypolicies\n  verbs:\n  - use\n  resourceNames:\n  - {{ template "sscd-psp.fullname" . }}\n{{- end }}\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role.yaml
	@sed -i '/^rules:/i \ \ labels:\n{{ include \"sscd.labels\" . | indent 4 }}' manifest_staging/charts/secrets-store-csi-driver/templates/role.yaml
	@sed -i '1s/^/{{ if .Values.rbac.install }}\n/gm; s/namespace: .*/namespace: {{ .Release.Namespace }}/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role_binding.yaml
	@sed -i '/^roleRef:/i \ \ labels:\n{{ include \"sscd.labels\" . | indent 4 }}' manifest_staging/charts/secrets-store-csi-driver/templates/role_binding.yaml
	@sed -i '1s/^/{{ if .Values.rbac.install }}\n/gm; s/namespace: .*/namespace: {{ .Release.Namespace }}/gm; $$s/$$/\n\ \ labels:\n{{ include "sscd.labels" . | indent 4 }}\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/serviceaccount.yaml

	# Generate secret syncing specific RBAC
	$(CONTROLLER_GEN) rbac:roleName=secretprovidersyncing-role paths="./controllers/syncsecret" output:dir=config/rbac-syncsecret
	$(KUSTOMIZE) build config/rbac-syncsecret -o manifest_staging/deploy/rbac-secretprovidersyncing.yaml
	cp config/rbac-syncsecret/role.yaml manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret.yaml
	cp config/rbac-syncsecret/role_binding.yaml manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret_binding.yaml
	@sed -i '1s/^/{{ if .Values.syncSecret.enabled }}\n/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret.yaml
	@sed -i '/^rules:/i \ \ labels:\n{{ include \"sscd.labels\" . | indent 4 }}' manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret.yaml
	@sed -i '1s/^/{{ if .Values.syncSecret.enabled }}\n/gm; s/namespace: .*/namespace: {{ .Release.Namespace }}/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret_binding.yaml
	@sed -i '/^roleRef:/i \ \ labels:\n{{ include \"sscd.labels\" . | indent 4 }}' manifest_staging/charts/secrets-store-csi-driver/templates/role-syncsecret_binding.yaml

	# Generate rotation specific RBAC
	$(CONTROLLER_GEN) rbac:roleName=secretproviderrotation-role paths="./pkg/rotation" output:dir=config/rbac-rotation
	$(KUSTOMIZE) build config/rbac-rotation -o manifest_staging/deploy/rbac-secretproviderrotation.yaml
	cp config/rbac-rotation/role.yaml manifest_staging/charts/secrets-store-csi-driver/templates/role-rotation.yaml
	cp config/rbac-rotation/role_binding.yaml manifest_staging/charts/secrets-store-csi-driver/templates/role-rotation_binding.yaml
	@sed -i '1s/^/{{ if .Values.enableSecretRotation }}\n/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role-rotation.yaml
	@sed -i '/^rules:/i \ \ labels:\n{{ include \"sscd.labels\" . | indent 4 }}' manifest_staging/charts/secrets-store-csi-driver/templates/role-rotation.yaml
	@sed -i '1s/^/{{ if .Values.enableSecretRotation }}\n/gm; s/namespace: .*/namespace: {{ .Release.Namespace }}/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role-rotation_binding.yaml
	@sed -i '/^roleRef:/i \ \ labels:\n{{ include \"sscd.labels\" . | indent 4 }}' manifest_staging/charts/secrets-store-csi-driver/templates/role-rotation_binding.yaml

	# Generate token requests specific RBAC
	$(CONTROLLER_GEN) rbac:roleName=secretprovidertokenrequest-role paths="./controllers/tokenrequest" output:dir=config/rbac-tokenrequest
	$(KUSTOMIZE) build config/rbac-tokenrequest -o manifest_staging/deploy/rbac-secretprovidertokenrequest.yaml
	cp config/rbac-tokenrequest/role.yaml manifest_staging/charts/secrets-store-csi-driver/templates/role-tokenrequest.yaml
	cp config/rbac-tokenrequest/role_binding.yaml manifest_staging/charts/secrets-store-csi-driver/templates/role-tokenrequest_binding.yaml
	@sed -i '1s/^/{{ if .Values.tokenRequests }}\n/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role-tokenrequest.yaml
	@sed -i '/^rules:/i \ \ labels:\n{{ include \"sscd.labels\" . | indent 4 }}' manifest_staging/charts/secrets-store-csi-driver/templates/role-tokenrequest.yaml
	@sed -i '1s/^/{{ if .Values.tokenRequests }}\n/gm; s/namespace: .*/namespace: {{ .Release.Namespace }}/gm; $$s/$$/\n{{ end }}/gm' manifest_staging/charts/secrets-store-csi-driver/templates/role-tokenrequest_binding.yaml
	@sed -i '/^roleRef:/i \ \ labels:\n{{ include \"sscd.labels\" . | indent 4 }}' manifest_staging/charts/secrets-store-csi-driver/templates/role-tokenrequest_binding.yaml

.PHONY: generate-protobuf
generate-protobuf: $(PROTOC) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC) # generates protobuf
	@PATH=$(PATH):$(TOOLS_BIN_DIR) $(PROTOC) -I . provider/v1alpha1/service.proto --go-grpc_out=require_unimplemented_servers=false:. --go_out=.
	# Update boilerplate for the generated file.
	cat hack/boilerplate.go.txt provider/v1alpha1/service_grpc.pb.go > tmpfile && mv tmpfile provider/v1alpha1/service_grpc.pb.go

## --------------------------------------
## Release
## --------------------------------------
.PHONY: release-manifest
release-manifest:
	$(MAKE) manifests
	@sed -i "s/version: .*/version: ${NEWVERSION}/" manifest_staging/charts/secrets-store-csi-driver/Chart.yaml
	@sed -i "s/appVersion: .*/appVersion: ${NEWVERSION}/" manifest_staging/charts/secrets-store-csi-driver/Chart.yaml
	@sed -i "s/tag: v${CURRENTVERSION}/tag: v${NEWVERSION}/" manifest_staging/charts/secrets-store-csi-driver/values.yaml
	@sed -i "s/v${CURRENTVERSION}/v${NEWVERSION}/" manifest_staging/charts/secrets-store-csi-driver/README.md
	@sed -i "s/driver:v${CURRENTVERSION}/driver:v${NEWVERSION}/" manifest_staging/deploy/secrets-store-csi-driver.yaml manifest_staging/deploy/secrets-store-csi-driver-windows.yaml

.PHONY: promote-staging-manifest
promote-staging-manifest: #promote staging manifests to release dir
	$(MAKE) release-manifest
	@rm -rf deploy
	@cp -r manifest_staging/deploy .
	@rm -rf charts/secrets-store-csi-driver
	@cp -r manifest_staging/charts/secrets-store-csi-driver ./charts

## --------------------------------------
## Local
## --------------------------------------
.PHONY: redeploy-driver
redeploy-driver: e2e-container
	kubectl delete pod $(shell kubectl get pod -n kube-system -l app=secrets-store-csi-driver -o jsonpath="{.items[0].metadata.name}") -n kube-system --force --grace-period 0
