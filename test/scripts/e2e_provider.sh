#!/usr/bin/env bash

echo "e2e provider test"
make e2e-bootstrap e2e-mock-provider-container e2e-helm-deploy e2e-provider-deploy e2e-provider
