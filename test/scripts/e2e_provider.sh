#!/usr/bin/env bash

echo "e2e provider provider test"
make e2e-bootstrap e2e-fake-provider-container e2e-helm-deploy e2e-provider-deploy e2e-provider