# sigs.k8s.io/controller-runtime/pkg/cache

The cache package has been forked from [`sigs.k8s.io/controller-runtime@a8c19c49e49cfba2aa486ff322cbe5222d6da533 (v0.8.2)`](https://github.com/kubernetes-sigs/controller-runtime/releases/tag/v0.8.2).

This fork has been modified to add the ability to perform filtered `ListWatch` based on the field or label selectors.

The original code for the cache package can be found at [https://github.com/kubernetes-sigs/controller-runtime/tree/v0.8.2/pkg/cache](https://github.com/kubernetes-sigs/controller-runtime/tree/v0.8.2/pkg/cache). We'll switch to using the default cache package in controller-runtime after this [Enable filtered list watches as watches](https://github.com/kubernetes-sigs/controller-runtime/issues/244) is implemented.
