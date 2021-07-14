ARG BASEIMAGE=mcr.microsoft.com/windows/nanoserver:1809
ARG BASEIMAGE_CORE=gcr.io/k8s-staging-e2e-test-images/windows-servercore-cache:1.0-linux-amd64-1809
ARG KUBERNETES_VERSION=1.21.1

FROM --platform=linux/amd64 ${BASEIMAGE_CORE} as core

FROM --platform=$BUILDPLATFORM golang:1.16 as builder
RUN curl -LO https://dl.k8s.io/release/v${KUBERNETES_VERSION}/bin/windows/amd64/kubectl.exe

FROM $BASEIMAGE
LABEL description="Secrets Store CSI Driver CRDs"
COPY * /crds/

COPY --from=builder /go/kubectl.exe /kubectl.exe
COPY --from=core /Windows/System32/netapi32.dll /Windows/System32/netapi32.dll
USER ContainerAdministrator
ENTRYPOINT ["/kubectl.exe"]
