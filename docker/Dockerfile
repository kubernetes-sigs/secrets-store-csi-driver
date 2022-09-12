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

ARG BASEIMAGE=k8s.gcr.io/build-image/debian-base:bullseye-v1.4.2

FROM golang:1.18 as builder
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
# upgrading zlib1g due to CVE-2022-37434
# upgrading libc-bin and libc6 due to CVE-2021-3999
# upgrading libpcre2-8-0 due to CVE-2022-1586, CVE-2022-1587
RUN clean-install ca-certificates mount zlib1g libc-bin libc6 libpcre2-8-0

LABEL maintainers="ritazh"
LABEL description="Secrets Store CSI Driver"

ENTRYPOINT ["/secrets-store-csi"]
