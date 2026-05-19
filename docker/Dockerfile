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

ARG BASEIMAGE=registry.k8s.io/build-image/debian-base:bookworm-v1.0.7@sha256:368abceecc1308e0913a6fd5ab89a513ee4268becefc2a82dbe616462b29a46b

FROM golang:1.25.10@sha256:315d6d304226d0180a011ead57af82c9fe16f84fcf0eaa17d11034a53f63fbc4 AS builder
WORKDIR /go/src/sigs.k8s.io/secrets-store-csi-driver
ADD . .
ARG TARGETARCH
ARG TARGETOS
ARG TARGETPLATFORM
ARG IMAGE_VERSION

RUN export GOOS=$TARGETOS && \
    export GOARCH=$TARGETARCH && \
    make build

FROM $BASEIMAGE
COPY --from=builder /go/src/sigs.k8s.io/secrets-store-csi-driver/_output/secrets-store-csi /secrets-store-csi
RUN apt update && \
    apt upgrade -y && \
    clean-install ca-certificates mount

LABEL description="Secrets Store CSI Driver"

ENTRYPOINT ["/secrets-store-csi"]
