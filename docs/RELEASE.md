# Release Process

## Overview

The release process consists of three phases: versioning, building and publishing

Versioning involves maintaining the following files:

- Makefile - the Makefile contains a `IMAGE_VERSION` variable that defines the version of the project.
- secrets-store-csi-driver.yaml - the Linux driver deployment yaml that contains the latest release tag image of the project.
- secrets-store-csi-driver-windows.yaml - the Windows driver deployment yaml that contains the latest release tag image of the project.
- `deploy/` dir that contains all the secrets-store-csi-driver resources to be deployed to the cluster including the latest release tag image of the project.
- `.github/workflows/e2e.yaml` file contains the default image tag released in staging and is used for e2e testing before releasing production images.

The steps below explain how to update these files. In addition, the repository should be tagged with the semantic version identifying the release.

Building involves obtaining a copy of the repository and triggering an automatic build as part of the [prow job](https://testgrid.k8s.io/sig-auth-secrets-store-csi-driver#secrets-store-csi-driver-push-image).

Publishing involves creating a release tag and creating a new Release on GitHub.

## Prerequisites

These steps require your `git remote` to be configured so that `origin` is your fork and `upstream` is `github.com/kubernetes-sigs/secrets-store-csi-driver`.

The `Makefile`s and scripts assume the gnu version of `sed`. This can be installed on OSX with `brew install gnu-sed`.

The scripts also require the `gh` tool, available from <https://github.com/cli/cli#installation>

## Versioning

1. Make sure that the `docs` include all necessary information included in the release (example [tag compare](https://github.com/kubernetes-sigs/secrets-store-csi-driver/compare/v0.3.0...main)).
1. Create a new release branch `release-X.X` using the UI (to avoid `git push`'ing directly to the repo).
1. Wait for the [new branch](https://github.com/kubernetes-sigs/secrets-store-csi-driver/branches) to receive [branch protection](https://docs.github.com/en/github/administering-a-repository/defining-the-mergeability-of-pull-requests/about-protected-branches).
1. Update the version to the semantic version of the new release (`Makefile` and `docker/Makefile`) similar to [this](https://github.com/kubernetes-sigs/secrets-store-csi-driver/pull/767).
1. Commit the changes and push to remote repository to create a pull request to the `release-X.X` branch

    ```bash
    git checkout <RELEASE_BRANCH>
    git checkout -b bump-version-<NEW_VERSION>
    git commit -m "release: bump version to <NEW_VERSION> in <RELEASE_BRANCH>"
    git push <YOUR FORK>
    ```

1. Once the PR is merged to `release-X.X`, the [prow job](https://testgrid.k8s.io/sig-auth-secrets-store-csi-driver#secrets-store-csi-driver-push-image) is triggered to build and push the new version to staging repo (`gcr.io/k8s-staging-csi-secrets-store/driver`)
1. After images are available in staging registry, head over to github [actions](https://github.com/kubernetes-sigs/secrets-store-csi-driver/actions) and select `e2e_mock_provider_tests` workflow. Click `Run workflow` and provide required inputs. This will run e2e tests against the staging images. Make sure this job is successful before proceeding to the next step.

    ```bash
    gh workflow run e2e_mock_provider_tests -f imageVersion=<NEW_VERSION>
    ```

1. Once the e2e tests on staging images completes, follow the [instructions](https://github.com/kubernetes/k8s.io/tree/main/registry.k8s.io#image-promoter) to promote the image to production repo
    - Within the Prow job "Artifacts" tab there is a file `artifacts/build.log` which will include the Cloud Build URL:

    ```text
    Created [https://cloudbuild.googleapis.com/v1/projects/k8s-staging-csi-secrets-store/locations/global/builds/<number>].
    ```

    - Run generate script to append the new image to promoter manifest

    ```bash
    registry.k8s.io/images/k8s-staging-csi-secrets-store/generate.sh > registry.k8s.io/images/k8s-staging-csi-secrets-store/images.yaml
    ```

    - Preview the changes

    ```bash
    git diff
    ```

    - Commit the changes and push to remote repository to create a pull request
1. Once the image promoter PR is merged, the image will be promoted to prod repo (`registry.k8s.io/csi-secrets-store/driver`)

## Building and releasing

1. Modify the `Makefile`s to include the changes from the `Version` section above.

1. Execute the promote-staging-manifest target to generate patch and promote staging manifest to release

    ```bash
   make promote-staging-manifest NEWVERSION=0.0.12 CURRENTVERSION=0.0.11
    ```

1. Preview the changes

    ```bash
   git diff
    ```

1. Commit the changes and push to remote repository to create a pull request

    ```bash
    git checkout -b release-<NEWVERSION> # i.e. release-0.3.0
    git commit -a -s -m "release: update manifests and helm chart for <NEWVERSION>"
    git push <YOUR FORK>
    ```

1. Create a cherry pick of the commit to the `release-X.X` branch:

    ```bash
    export GITHUB_USER=<user name>
    hack/cherry_pick_pull.sh upstream/<release branch> <pr number>
    ```

1. Once the PR is merged to `release-X.X` we are ready to tag `release-X.X` with release
   version.

## Publishing

> The following steps can only be performed by the maintainers of the repository.

Clone the repository and tag the release with the semantic version of the release.

 ```bash
 git clone git@github.com:kubernetes-sigs/secrets-store-csi-driver.git
 cd secrets-store-csi-driver
 git checkout <RELASE_BRANCH>
 git tag -a v<NEW_VERSION> -m "release: <NEW_VERSION>" # NEW_VERSION is the semantic version of the release and will contain the v prefix. e.g. v1.0.0
 git push origin --tags
 ```

> Delete the clone of the repository from local machine to prevent accidental actions on the main repository.

After the tag is created, [Create release](https://github.com/kubernetes-sigs/secrets-store-csi-driver/actions/workflows/create-release.yaml) action will be triggered to create a release on GitHub. This workflow will create a release with the deployment manifests and generate a changelog.
