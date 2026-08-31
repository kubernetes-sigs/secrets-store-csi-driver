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

ARG BASEIMAGE=registry.k8s.io/build-image/debian-base:bookworm-v1.0.8@sha256:bb933ac3606ff3f69854a7eba5da1eaef5064ef62d83f3cb0e4be40eb53522e0

FROM golang:1.26.4@sha256:68cb6d68bed024785b69195b89af7ac7a444f27791435f98647edff595aa0479 AS builder
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

# Deliberately not installing ca-certificates: debian-base already ships
# /etc/ssl/certs/ca-certificates.crt, copied in without the package so that
# openssl is not pulled in as a dependency. Re-installing the package here would
# undo that and add openssl + libssl3 back to the image. The driver is built with
# CGO_ENABLED=0, so it reads the trust bundle as a file via crypto/x509 and never
# links libssl3 nor execs the openssl CLI.
RUN apt update && \
    apt upgrade -y && \
    clean-install mount

LABEL description="Secrets Store CSI Driver"

ENTRYPOINT ["/secrets-store-csi"]
