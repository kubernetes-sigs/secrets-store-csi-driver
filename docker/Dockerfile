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

ARG BASEIMAGE=k8s.gcr.io/build-image/debian-base:bullseye-v1.1.0

FROM golang:1.17 as builder
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
# upgrading libgmp10 due to CVE-2021-43618
# upgrading bsdutils due to CVE-2021-3995 and CVE-2021-3996
# upgrading libssl1.1 due to CVE-2022-0778 and CVE-2021-4160
# upgrading libc-bin due to CVE-2021-33574, CVE-2022-23218, CVE-2022-23219 and CVE-2021-43396
# upgrading libc6 due to CVE-2021-33574, CVE-2022-23218, CVE-2022-23219 and CVE-2021-43396
# upgrading libsystemd0 due to CVE-2021-3997
# upgrading libudev1 due to CVE-2021-3997
RUN clean-install ca-certificates mount libgmp10 bsdutils libssl1.1 libc-bin libc6 libsystemd0 libudev1

LABEL maintainers="ritazh"
LABEL description="Secrets Store CSI Driver"

ENTRYPOINT ["/secrets-store-csi"]
