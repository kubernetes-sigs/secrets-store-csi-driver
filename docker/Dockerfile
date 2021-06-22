ARG BASEIMAGE=us.gcr.io/k8s-artifacts-prod/build-image/debian-base:buster-v1.7.2

FROM golang:1.16 as builder
WORKDIR /go/src/sigs.k8s.io/secrets-store-csi-driver
ADD . .
ARG TARGETARCH
ARG TARGETOS
ARG TARGETPLATFORM

RUN export GOOS=$TARGETOS && \
    export GOARCH=$TARGETARCH && \
    make build

FROM $BASEIMAGE
COPY --from=builder /go/src/sigs.k8s.io/secrets-store-csi-driver/_output/secrets-store-csi /secrets-store-csi
# upgrading libgcrypt20 due to CVE-2021-33560
# upgrading libgnutls30 due to CVE-2021-20231, CVE-2021-20232, CVE-2020-24659
# upgrading libhogweed4 due to CVE-2021-20305, CVE-2021-3580
# upgrading libnettle6 due to CVE-2021-20305, CVE-2021-3580
RUN clean-install ca-certificates mount libgcrypt20 libgnutls30 libhogweed4 libnettle6

LABEL maintainers="ritazh"
LABEL description="Secrets Store CSI Driver"

ENTRYPOINT ["/secrets-store-csi"]
