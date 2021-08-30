#!/usr/bin/env bash

echo "e2e provider test"
# make e2e-bootstrap e2e-mock-provider-container e2e-helm-deploy e2e-provider-deploy e2e-provider
make e2e-bootstrap e2e-mock-provider-container e2e-provider-deploy

helm install csi-secrets-store manifest_staging/charts/secrets-store-csi-driver --namespace kube-system --wait --timeout=5m -v=5 --debug \
		--set linux.image.pullPolicy="IfNotPresent" \
		--set windows.image.pullPolicy="IfNotPresent" \
		--set linux.crds.annotations."myAnnotation"=test \
		--set windows.enabled=true \
		--set linux.enabled=true \
		--set syncSecret.enabled=true \
		--set enableSecretRotation=true \
		--set rotationPollInterval=60s

make e2e-provider