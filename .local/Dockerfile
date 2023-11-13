FROM golang:1.21-alpine@sha256:4f95f6bd37a96bb17ff610ed3bb424fc7d2926e08da4ed2276ed5f279d377852

ENV CGO_ENABLED=0
ENV GOROOT=/usr/local/go
ENV GOPATH=${HOME}/go
ENV PATH=$PATH:${GOROOT}/bin

RUN apk update && apk add --no-cache \
    git && \
    go get github.com/go-delve/delve/cmd/dlv

WORKDIR /secrets-store-csi-driver-codebase

COPY go.mod go.mod
RUN go mod download

EXPOSE 30123

# these dlv debug arguments replicate driver args from DaemonSet
ENTRYPOINT ["/go/bin/dlv", "--listen=:30123", "--accept-multiclient", "--headless=true", "--api-version=2", "debug", "./cmd/secrets-store-csi-driver", "--", "-v", "5", "-endpoint", "unix:///csi/csi.sock", "-nodeid", "kind-control-plane", "-enable-secret-rotation", "false", "-rotation-poll-interval", "30s", "-metrics-addr", ":8080", "-provider-volume", "/etc/kubernetes/secrets-store-csi-providers"]
