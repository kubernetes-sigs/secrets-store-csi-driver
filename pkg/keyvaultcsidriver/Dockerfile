FROM debian:9
RUN apt-get update && apt-get install -y ca-certificates cifs-utils
LABEL maintainers="ritazh"
LABEL description="Keyvault CSI Driver"

COPY ./_output/keyvaultcsi /keyvaultcsi
ENTRYPOINT ["/keyvaultcsi"]
