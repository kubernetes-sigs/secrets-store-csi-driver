FROM mcr.microsoft.com/windows/servercore:1809 as core

FROM mcr.microsoft.com/windows/nanoserver:1809
LABEL description="Secrets Store CSI Driver"

COPY ./_output/secrets-store-csi.exe /secrets-store-csi.exe
COPY --from=core /Windows/System32/netapi32.dll /Windows/System32/netapi32.dll
USER ContainerAdministrator
ENTRYPOINT ["/secrets-store-csi.exe"]
