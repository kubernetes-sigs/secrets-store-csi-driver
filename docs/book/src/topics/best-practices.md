# Best Practices

1. Deploy the driver and providers into the `kube-system` or a separate
  dedicated namespace.

    The driver is installed as a `DaemonSet` with the ability to mount kubelet
    `hostPath` volumes and view pod service account tokens. It should be treated
    as privileged and regular cluster users should not have permissions to
    deploy or modify the driver.

1. Do not grant regular cluster users permissions to modify
  `SecretProviderClassPodStatus` resources.

    The `SecretProviderClassPodStatus` CRD is used by the driver to keep track
    of mounted resources. Manually editing this resource could have unexpected
    consequences to the system health and in particular modifying
    `SecretProviderClassPodStatus/status` may have security implications.

1. Disable `Secret` sync if not needed.

    If you do not intend to use the `Secret` syncing feature, do not install the
    RBAC permissions that allow the driver to access cluster `Secret` objects.

    This can be done by setting `syncSecret.enabled = false` when installing
    with helm.

1. Enable KMS application wrapping if using `Secret` sync.

    If you need to synchronise your external secrets to Kubernetes `Secret`s
    consider configuring
    [encryption of data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/)

    This will ensure that data is encrypted before it is stored in `etcd`.

1. Keep the driver up to date.

    Subscribe to the
    [`kubernetes-secrets-store-csi-driver`](https://groups.google.com/forum/#!forum/kubernetes-secrets-store-csi-driver)
    mailing list to be notified of new releases and security announcements.

    Consider using the
    [Github Watch](https://docs.github.com/en/github/managing-subscriptions-and-notifications-on-github/viewing-your-subscriptions)
    feature to subscribe to releases as well.

    Always be sure to review the [release notes](https://github.com/kubernetes-sigs/secrets-store-csi-driver/releases)
    before upgrading.

1. When evaluating this driver consider the following threats:

    * When a secret is accessible on the **filesystem**, application
      vulnerabilities like directory traversal attacks can become higher
      severity as the attacker may gain the ability read the secret material.
    * When a secret is consumed through **environment variables**,
      misconfigurations such as enabling a debug endpoints
      or including dependencies that log process environment details may leak
      secrets.
    * When syncing secret material to Kubernetes Secrets, consider whether the
      access controls on that data store are sufficiently narrow in scope.

    If possible, directly integrating with a purpose built secrets API may offer
    the best security tradeoffs.
