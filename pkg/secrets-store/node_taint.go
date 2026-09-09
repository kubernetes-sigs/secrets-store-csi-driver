/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package secretsstore

import (
	"context"
	"encoding/json"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// AgentNotReadyNodeTaintKey is the taint key the driver removes from its local
// node once it has started. Operators (for example EKS Managed Node Groups) can
// apply this taint to a node so that workload pods are not scheduled there until
// the secrets-store CSI driver pod is running, avoiding the startup race where a
// pod mounts a SecretProviderClass volume before the driver is ready.
const AgentNotReadyNodeTaintKey = "secrets-store.csi.k8s.io/agent-not-ready"

// nodeNameEnvVar is the environment variable the DaemonSet populates from
// spec.nodeName (also consumed as --nodeid). It identifies the local node whose
// taint should be removed.
const nodeNameEnvVar = "KUBE_NODE_NAME"

// kubernetesClientGetter lazily builds a Kubernetes clientset. It is a function
// so tests can inject a fake clientset and exercise the failure path.
type kubernetesClientGetter func() (kubernetes.Interface, error)

// inClusterClientGetter builds a clientset from the in-cluster config. It is the
// production implementation passed to removeTaintInBackground.
var inClusterClientGetter kubernetesClientGetter = func() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

// taintRemovalBackoff is the exponential backoff for node taint removal retries.
// Max delay across the steps is 0.5s * 2^9 = ~4 minutes.
var taintRemovalBackoff = wait.Backoff{
	Duration: 500 * time.Millisecond,
	Factor:   2,
	Steps:    10,
}

// jsonPatch is a single RFC 6902 JSON patch operation.
type jsonPatch struct {
	OP    string      `json:"op,omitempty"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value"`
}

// RemoveNotReadyTaintInBackground removes the node startup taint
// (AgentNotReadyNodeTaintKey) from the local node in a background goroutine,
// retrying with exponential backoff. It is the production entry point and is safe
// to call unconditionally: it no-ops on clusters that never apply the taint.
func RemoveNotReadyTaintInBackground() {
	go removeTaintInBackground(inClusterClientGetter, removeNotReadyTaint)
}

// removeTaintInBackground retries removalFunc with exponential backoff until it
// succeeds or the backoff is exhausted. It is intended to be run as a goroutine
// so driver startup is not blocked on the Kubernetes API being reachable.
func removeTaintInBackground(clientGetter kubernetesClientGetter, removalFunc func(kubernetesClientGetter) error) {
	backoffErr := wait.ExponentialBackoff(taintRemovalBackoff, func() (bool, error) {
		if err := removalFunc(clientGetter); err != nil {
			klog.ErrorS(err, "Unexpected failure when attempting to remove node taint(s)")
			return false, nil
		}
		return true, nil
	})
	if backoffErr != nil {
		klog.ErrorS(backoffErr, "Retries exhausted, giving up attempting to remove node taint(s)")
	}
}

// removeNotReadyTaint removes the AgentNotReadyNodeTaintKey taint from the local
// node. It is a no-op (returns nil) when the node name is unknown or the client
// cannot be built, so a cluster that never applies the taint is unaffected. It
// returns an error only on transient API failures so the caller can retry.
func removeNotReadyTaint(clientGetter kubernetesClientGetter) error {
	nodeName := os.Getenv(nodeNameEnvVar)
	if nodeName == "" {
		klog.V(4).InfoS("node name env var missing, skipping taint removal", "envVar", nodeNameEnvVar)
		return nil
	}

	clientset, err := clientGetter()
	if err != nil {
		klog.V(4).InfoS("failed to setup k8s client, skipping taint removal", "error", err)
		return nil
	}

	node, err := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	taintsToKeep := make([]corev1.Taint, 0, len(node.Spec.Taints))
	for _, taint := range node.Spec.Taints {
		if taint.Key != AgentNotReadyNodeTaintKey {
			taintsToKeep = append(taintsToKeep, taint)
		} else {
			klog.V(4).InfoS("queued taint for removal", "key", taint.Key, "effect", taint.Effect)
		}
	}

	if len(taintsToKeep) == len(node.Spec.Taints) {
		klog.V(4).InfoS("no taints to remove on node, skipping taint removal", "node", nodeName)
		return nil
	}

	// Use a test-and-replace patch so we never clobber a concurrent update to
	// the node's taints: if spec.taints changed since the Get, the test op fails
	// and the patch is rejected, and the backoff retry re-reads the node.
	patchRemoveTaints := []jsonPatch{
		{
			OP:    "test",
			Path:  "/spec/taints",
			Value: node.Spec.Taints,
		},
		{
			OP:    "replace",
			Path:  "/spec/taints",
			Value: taintsToKeep,
		},
	}

	patch, err := json.Marshal(patchRemoveTaints)
	if err != nil {
		return err
	}

	if _, err := clientset.CoreV1().Nodes().Patch(context.Background(), nodeName, k8stypes.JSONPatchType, patch, metav1.PatchOptions{}); err != nil {
		return err
	}

	klog.InfoS("removed taint(s) from local node", "node", nodeName, "taintKey", AgentNotReadyNodeTaintKey)
	return nil
}
