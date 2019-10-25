FROM debian:9
RUN apt-get update && apt-get install -y ca-certificates cifs-utils
LABEL maintainers="ritazh"
LABEL description="Secrets Store CSI Driver"

COPY ./_output/secrets-store-csi /secrets-store-csi
ENTRYPOINT ["/secrets-store-csi"]
