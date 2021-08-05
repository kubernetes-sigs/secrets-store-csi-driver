FROM golang:1.16 as builder
WORKDIR /go/src/sigs.k8s.io/secrets-store-csi-driver
ADD . .
RUN make build-e2e-provider
RUN ls -latr test/e2eprovider

FROM gcr.io/distroless/static
COPY --from=builder /go/src/sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider/e2e-provider /e2e-provider

LABEL maintainers="nilekhc"
LABEL description="Fake provider for Secrets Store CSI Driver"

ENTRYPOINT ["/e2e-provider"]
