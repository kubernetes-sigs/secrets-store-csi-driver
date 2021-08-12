# Release Management

## Overview
This document describes **Kubernetes Secrets Store CSI Driver** project release management, which talks about versioning, branching and cadence.

## Legend

- **X.Y.Z** refers to the version (git tag) of Secrets Store CSI Driver that is released. This is the version of the Secrets Store CSI Driver image.
 
- **Milestone** should be designed to include feature sets to accommodate monthly release cycles including test gates. GitHub milestones are used by maintainers to manage each release. PRs and Issues for each release should be created as part of a corresponding milestone.

- **Test gates** should include soak tests and upgrade tests from the last minor version.

## Versioning
This project strictly follows [semantic versioning](https://semver.org/spec/v2.0.0.html). All releases will be of the form _vX.Y.Z_ where X is the major version, Y is the minor version and Z is the patch version.

### Patch releases
- Patch releases provide users with bug fixes and security fixes. They do not contain new features.

### Minor releases
- Minor releases contain security and bug fixes as well as _**new features**_.

- They are backwards compatible.

### Major releases
- Major releases contain breaking changes. Breaking changes refer to schema changes and behavior changes of Secrets Store CSI Driver that may require a clean install during upgrade and it may introduce changes that could break backward compatibility.

- Ideally we will avoid making multiple major releases to be always backward compatible, unless project evolves in important new directions and such release is necessary.

- Secrets Store CSI Driver is currently tracking towards first stable release(v1.0.0) with [this](https://github.com/kubernetes-sigs/secrets-store-csi-driver/milestone/5) milestone.

## Release Cadence and Branching
- Secrets Store CSI Driver follows `monthly` release schedule.

- A new release should be created in _`second week`_ of each month. This schedule not only allows us to do bug fixes, but also provides an opportunity to address underline image vulnerabilities etc. if any.

- The new version is decided as per above guideline and release branch should be created from `master` with name `release-<version>`. For eg. `release-0.1`. Then build the image from release branch.

- Any `fixes` or `patches` should be merged to master and then `cherry pick` to the release branch.

## Acknowledgement

This document builds on the ideas and implementations of release processes from projects like [Gatekeeper](https://github.com/open-policy-agent/gatekeeper/blob/master/docs/Release_Management.md), [Helm](https://helm.sh/docs/topics/release_policy/#helm) and Kubernetes. 
