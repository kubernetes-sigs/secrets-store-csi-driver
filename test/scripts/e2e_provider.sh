#!/usr/bin/env bash

echo "e2e provider test"
ROTATION_POLL_INTERVAL=120s make e2e-bootstrap e2e-mock-provider-container e2e-helm-deploy e2e-provider-deploy e2e-provider
