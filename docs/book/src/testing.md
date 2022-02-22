# Testing

## Unit Tests

Run unit tests locally with `make test`.

## End-to-end Tests

End-to-end tests automatically runs on Prow when a PR is submitted. If you want to run using a local or remote Kubernetes cluster, make sure to have `kubectl`, `helm` and `bats` set up in your local environment and then run `make e2e-azure`, `make e2e-vault`, `make e2e-akeyless` or `make e2e-gcp` with custom images.

Job config for test jobs run for each PR in prow can be found [here](https://github.com/kubernetes/test-infra/blob/master/config/jobs/kubernetes-sigs/secrets-store-csi-driver/secrets-store-csi-driver-config.yaml)
