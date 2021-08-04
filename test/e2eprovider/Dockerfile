FROM golang:1.16 as builder
WORKDIR /go/src/sigs.k8s.io/secrets-store-csi-driver
ADD . .
RUN make build-fake-provider
RUN ls -latr test/e2eprovider

FROM gcr.io/distroless/static
COPY --from=builder /go/src/sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider/fake-provider /fake-provider

LABEL maintainers="nilekhc"
LABEL description="Fake provider for Secrets Store CSI Driver"

ENTRYPOINT ["/fake-provider"]
