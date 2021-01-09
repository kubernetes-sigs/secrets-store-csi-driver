ARG BASEIMAGE=us.gcr.io/k8s-artifacts-prod/build-image/debian-base-amd64:buster-v1.2.0

FROM golang:1.13.10-alpine3.10 as builder
WORKDIR /go/src/sigs.k8s.io/secrets-store-csi-driver
ADD . .
ARG TARGETARCH
ARG TARGETOS
ARG LDFLAGS
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -ldflags "${LDFLAGS}" -o _output/secrets-store-csi ./cmd/secrets-store-csi-driver

FROM $BASEIMAGE
COPY --from=builder /go/src/sigs.k8s.io/secrets-store-csi-driver/_output/secrets-store-csi /secrets-store-csi
# upgrading apt &libapt-pkg5.0 due to CVE-2020-27350
# upgrading libp11-kit0 due to CVE-2020-29362, CVE-2020-29363 and CVE-2020-29361
RUN apt-mark unhold apt && \
    clean-install ca-certificates cifs-utils mount apt libapt-pkg5.0 libp11-kit0 wget

LABEL maintainers="ritazh"
LABEL description="Secrets Store CSI Driver"

ENTRYPOINT ["/secrets-store-csi"]
