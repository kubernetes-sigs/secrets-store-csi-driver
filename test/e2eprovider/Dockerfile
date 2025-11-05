# Copyright 2021 The Kubernetes Authors.
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

FROM golang:1.25.3@sha256:6d4e5e74f47db00f7f24da5f53c1b4198ae46862a47395e30477365458347bf2 as builder
WORKDIR /go/src/sigs.k8s.io/secrets-store-csi-driver
ADD . .
RUN make build-e2e-provider

FROM gcr.io/distroless/static
COPY --from=builder /go/src/sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider/e2e-provider /e2e-provider

LABEL maintainers="nilekhc"
LABEL description="Mock provider for Secrets Store CSI Driver"

ENTRYPOINT ["/e2e-provider"]
