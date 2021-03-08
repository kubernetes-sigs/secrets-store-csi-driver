ARG BASEIMAGE=mcr.microsoft.com/windows/nanoserver:1809
ARG BASEIMAGE_CORE=mcr.microsoft.com/windows/servercore:1809

FROM --platform=$BUILDPLATFORM golang:1.16-alpine as builder
WORKDIR /go/src/sigs.k8s.io/secrets-store-csi-driver
ADD . .
ARG TARGETARCH
ARG TARGETOS
ARG LDFLAGS
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -ldflags "${LDFLAGS}" -o _output/secrets-store-csi.exe ./cmd/secrets-store-csi-driver

FROM mcr.microsoft.com/windows/servercore:1809 as core

FROM $BASEIMAGE_CORE as core
FROM $BASEIMAGE
LABEL description="Secrets Store CSI Driver"

COPY --from=builder /go/src/sigs.k8s.io/secrets-store-csi-driver/_output/secrets-store-csi.exe /secrets-store-csi.exe
COPY --from=core /Windows/System32/netapi32.dll /Windows/System32/netapi32.dll
USER ContainerAdministrator
ENTRYPOINT ["/secrets-store-csi.exe"]
