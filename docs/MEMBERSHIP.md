# Secrets Store CSI Driver Project Membership

> Note: This document is a work in progress

> Original doc: [Secrets Store CSI Driver Project Membership](https://docs.google.com/document/d/1YqFVTJdxvNXhXkWogYf7AOKMnUjPFHCPuYtTgJSHohU/edit?usp=sharing)

## Overview

This document outlines the various responsibilities for the Secrets Store CSI Driver sig-auth subproject.

### Reviewer

Reviewers are able to review code for quality and correctness on the Secrets Store CSI Driver. They are knowledgeable about the codebase and software engineering principles.

**Defined by:** reviewers entry in the [OWNERS](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/v0.0.22/OWNERS#L5) file

**Note:** Acceptance of code contributions requires at least one approver in addition to the assigned reviewers.

#### Reviewer requirements

The following apply to the Secrets Store CSI Driver subproject for which one would be a reviewer in the OWNERS file.

- Member of kubernetes-sigs org
- Reviewer for at least 5 PRs to the codebase
- Reviewed or merged at least 10 substantial PRs to the codebase
- Knowledgeable about the codebase
- Sponsored by a subproject approver
  - With no objections from other approvers
  - Done through PR to update the OWNERS file
- May either self-nominate, be nominated by an approver in this subproject.

#### Reviewer responsibilities and privileges

- Tests are run for PRs from members who aren’t part of Kubernetes org
  - This involves running ok-to-test after assessing the PR briefly
- Responsible for project quality control via code reviews
  - Focus on code quality and correctness, including testing and factoring
  - May also review for more holistic issues, but not a requirement
- Expected to be responsive to review requests as per [community expectations](https://github.com/kubernetes/community/blob/master/contributors/guide/expectations.md)
- Assigned PRs to review
- Assigned test bugs

### Approver

Code approvers are able to both review and approve code contributions. While code review is focused on code quality and correctness, approval is focused on holistic acceptance of a contribution including: backwards / forwards compatibility, adhering to API and flag conventions, subtle performance and correctness issues, interactions with other parts of the system, etc.

**Defined by:** approvers entry in the [OWNERS](https://github.com/kubernetes-sigs/secrets-store-csi-driver/blob/v0.0.22/OWNERS#L1) file

#### Approver requirements

The following apply to the Secrets Store CSI Driver subproject for which one would be an approver in the OWNERS file.

- Reviewer of the codebase for at least 3 months
- Primary reviewer for at least 10 substantial PRs to the codebase
- Reviewed or merged at least 30 PRs to the codebase
- Nominated by a subproject owner
  - With no objections from other subproject owners
  - Done through PR to update the top-level OWNERS file

#### Approver Responsibilities and privileges

- Approver status may be a precondition to accepting large code contributions
- Demonstrate sound technical judgement
- Responsible for project quality control via code reviews
  - Focus on holistic acceptance of contribution such as dependencies with other features, backwards / forwards compatibility, API and flag definitions, etc
- Expected to be responsive to review requests as per community expectations
- Mentor contributors and reviewers
- May approve code contributions for acceptance

### Docs used for reference

- [Community membership](https://github.com/kubernetes/community/blob/master/community-membership.md)
