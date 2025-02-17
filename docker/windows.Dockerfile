# Copyright 2019 The Kubernetes Authors.
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

ARG BASEIMAGE=mcr.microsoft.com/windows/nanoserver:1809
ARG BASEIMAGE_CORE=gcr.io/k8s-staging-e2e-test-images/windows-servercore-cache:1.0-linux-amd64-1809

FROM --platform=linux/amd64 ${BASEIMAGE_CORE} as core

FROM --platform=$BUILDPLATFORM golang:1.24 as builder
WORKDIR /go/src/sigs.k8s.io/secrets-store-csi-driver
ADD . .
ARG TARGETARCH
ARG TARGETOS
ARG IMAGE_VERSION

RUN export GOOS=$TARGETOS && \
    export GOARCH=$TARGETARCH && \
    make build-windows

FROM $BASEIMAGE
LABEL description="Secrets Store CSI Driver"

COPY --from=builder /go/src/sigs.k8s.io/secrets-store-csi-driver/_output/secrets-store-csi.exe /secrets-store-csi.exe
COPY --from=core /Windows/System32/netapi32.dll /Windows/System32/netapi32.dll
USER ContainerAdministrator
ENTRYPOINT ["/secrets-store-csi.exe"]
