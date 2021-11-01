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

FROM alpine as builder
ARG KUBE_VERSION=v1.21.2
ARG TARGETARCH

RUN apk add --no-cache curl && \
    curl -LO https://storage.googleapis.com/kubernetes-release/release/${KUBE_VERSION}/bin/linux/${TARGETARCH}/kubectl && \
    chmod +x kubectl

FROM gcr.io/distroless/static
COPY * /crds/
COPY --from=builder /kubectl /kubectl
ENTRYPOINT ["/kubectl"]
