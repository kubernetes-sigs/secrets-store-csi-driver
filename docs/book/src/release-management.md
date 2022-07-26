# Release Management

<!-- toc -->

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

## Release Cadence and Branching

- Secrets Store CSI Driver follows `monthly` release schedule.

- A new release should be created in _`second week`_ of each month. This schedule not only allows us to do bug fixes, but also provides an opportunity to address underline image vulnerabilities etc. if any.

- The new version is decided as per above guideline and release branch should be created from `main` with name `release-<version>`. For eg. `release-0.1`. Then build the image from release branch.

- Any `fixes` or `patches` should be merged to main and then `cherry pick` to the release branch.

## Security Vulnerabilities

We use [trivy](https://github.com/aquasecurity/trivy) to scan the base image for known vulnerabilities. When a vulnerability is detected and has a fixed version, we will update the image to include the fix. For vulnerabilities that are not in a fixed version, there is nothing that can be done immediately. 
Fixable CVE patches will be part of the patch releases published **second week of every month**.

## Supported Releases

Applicable fixes, including security fixes, may be cherry-picked into the release branch, depending on severity and feasibility. Patch releases are cut from that branch as needed.

We expect users to stay reasonably up-to-date with the versions of Secrets Store CSI Driver they use in production, but understand that it may take time to upgrade. We expect users to be running approximately the latest patch release of a given minor release and encourage users to upgrade as soon as possible.

We expect to "support" n (current) and n-1 major.minor releases. "Support" means we expect users to be running that version in production. For example, when v1.3.0 comes out, v1.1.x will no longer be supported for patches and we encourage users to upgrade to a supported version as soon as possible.

## Supported Kubernetes Versions

Secrets Store CSI Driver will maintain support for all actively supported Kubernetes minor releases per [Kubernetes Supported Versions policy](https://kubernetes.io/releases/version-skew-policy/). If you choose to use Secrets Store CSI Driver with a version of Kubernetes that it does not support, you are using it at your own risk.

## Acknowledgement

This document builds on the ideas and implementations of release processes from projects like [Gatekeeper](https://github.com/open-policy-agent/gatekeeper/blob/master/docs/Release_Management.md), [Helm](https://helm.sh/docs/topics/release_policy/#helm) and Kubernetes.
