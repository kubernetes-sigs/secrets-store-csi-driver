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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const testNodeName = "test-node-123"

// node returns a *corev1.Node named testNodeName with the supplied taints.
func node(taints ...corev1.Taint) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec:       corev1.NodeSpec{Taints: taints},
	}
}

func TestRemoveNotReadyTaint(t *testing.T) {
	otherTaint := corev1.Taint{Key: "example.com/other", Effect: corev1.TaintEffectNoSchedule}
	notReadyTaint := corev1.Taint{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute}

	testCases := []struct {
		name string
		// setNodeNameEnv controls whether KUBE_NODE_NAME is set for the test.
		setNodeNameEnv bool
		// objects seeds the fake clientset.
		objects []runtime.Object
		// reactors are prepended so they take precedence over default tracker behavior.
		reactors []k8stesting.Reactor
		// clientGetterErr, when set, makes the client getter return an error.
		clientGetterErr error
		expErr          bool
		// expectPatch asserts whether a patch was issued against the node.
		expectPatch bool
		// expectTaintsAfter is checked against the live node when expectPatch is true.
		expectTaintsAfter []corev1.Taint
	}{
		{
			name:           "missing KUBE_NODE_NAME is a no-op",
			setNodeNameEnv: false,
			expErr:         false,
			expectPatch:    false,
		},
		{
			name:            "client getter failure is a no-op",
			setNodeNameEnv:  true,
			clientGetterErr: fmt.Errorf("failed to build client"),
			expErr:          false,
			expectPatch:     false,
		},
		{
			name:           "get node failure returns error",
			setNodeNameEnv: true,
			objects:        nil, // node absent -> Get returns NotFound
			expErr:         true,
			expectPatch:    false,
		},
		{
			name:           "node with no taints does not patch",
			setNodeNameEnv: true,
			objects:        []runtime.Object{node()},
			expErr:         false,
			expectPatch:    false,
		},
		{
			name:           "node without the not-ready taint does not patch",
			setNodeNameEnv: true,
			objects:        []runtime.Object{node(otherTaint)},
			expErr:         false,
			expectPatch:    false,
		},
		{
			name:           "patch failure returns error",
			setNodeNameEnv: true,
			objects:        []runtime.Object{node(notReadyTaint)},
			reactors: []k8stesting.Reactor{
				&k8stesting.SimpleReactor{
					Verb:     "patch",
					Resource: "nodes",
					Reaction: func(action k8stesting.Action) (bool, runtime.Object, error) {
						return true, nil, fmt.Errorf("failed to patch node")
					},
				},
			},
			expErr:      true,
			expectPatch: false,
		},
		{
			name:              "removes only the not-ready taint, keeps others",
			setNodeNameEnv:    true,
			objects:           []runtime.Object{node(notReadyTaint, otherTaint)},
			expErr:            false,
			expectPatch:       true,
			expectTaintsAfter: []corev1.Taint{otherTaint},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setNodeNameEnv {
				t.Setenv(nodeNameEnvVar, testNodeName)
			} else {
				// Ensure the var is empty even if inherited from the environment.
				t.Setenv(nodeNameEnvVar, "")
			}

			cs := k8sfake.NewSimpleClientset(tc.objects...)
			if len(tc.reactors) > 0 {
				cs.ReactionChain = append(tc.reactors, cs.ReactionChain...)
			}

			getter := func() (kubernetes.Interface, error) {
				if tc.clientGetterErr != nil {
					return nil, tc.clientGetterErr
				}
				return cs, nil
			}

			err := removeNotReadyTaint(getter)
			if tc.expErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			if tc.expectPatch {
				updated, getErr := cs.CoreV1().Nodes().Get(t.Context(), testNodeName, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, tc.expectTaintsAfter, updated.Spec.Taints)
			}
		})
	}
}

// TestRemoveNotReadyTaintSoleTaintSerializesAsEmptyArray verifies that when the
// not-ready taint is the ONLY taint, the replace op serializes the resulting
// empty taint list as a JSON empty array ([]) and not null. A null would clear
// the field in a way the apiserver treats differently from an explicit empty
// list, and a future regression to a nil slice would emit null. The patch bytes
// are captured to assert the wire format directly.
func TestRemoveNotReadyTaintSoleTaintSerializesAsEmptyArray(t *testing.T) {
	t.Setenv(nodeNameEnvVar, testNodeName)
	notReadyTaint := corev1.Taint{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute}

	cs := k8sfake.NewSimpleClientset(node(notReadyTaint))

	var capturedPatch []byte
	cs.PrependReactor("patch", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		capturedPatch = action.(k8stesting.PatchAction).GetPatch()
		// Fall through to the default tracker so the patch is actually applied.
		return false, nil, nil
	})

	err := removeNotReadyTaint(func() (kubernetes.Interface, error) { return cs, nil })
	assert.NoError(t, err)

	// The replace op must serialize the (now empty) taint list as [] not null.
	// The test op's value is the pre-read non-empty taint list, so `"value":[]`
	// uniquely identifies the replace op.
	assert.Contains(t, string(capturedPatch), `"value":[]`)
	assert.NotContains(t, string(capturedPatch), `"value":null`)

	updated, getErr := cs.CoreV1().Nodes().Get(t.Context(), testNodeName, metav1.GetOptions{})
	assert.NoError(t, getErr)
	assert.Empty(t, updated.Spec.Taints)
}

// TestRemoveNotReadyTaintConcurrentModificationRejected proves the RFC6902 test
// op actually guards against a concurrent taint change between the Get and the
// Patch. The client Get returns a stale view (only the not-ready taint) while the
// live node carries an additional taint, so the test op's asserted value differs
// from live state and the patch must be rejected. removeNotReadyTaint surfaces
// that as an error, which removeTaintInBackground retries.
func TestRemoveNotReadyTaintConcurrentModificationRejected(t *testing.T) {
	t.Setenv(nodeNameEnvVar, testNodeName)
	notReadyTaint := corev1.Taint{Key: AgentNotReadyNodeTaintKey, Effect: corev1.TaintEffectNoExecute}
	otherTaint := corev1.Taint{Key: "example.com/other", Effect: corev1.TaintEffectNoSchedule}

	// Live tracker state carries both taints.
	cs := k8sfake.NewSimpleClientset(node(notReadyTaint, otherTaint))

	// Stale client Get returns only the not-ready taint, so the code computes a
	// test-op value of [notReadyTaint], which will not match the live
	// /spec/taints ([notReadyTaint, otherTaint]) at patch time. The patch's
	// internal read goes through the tracker directly, not this reaction chain.
	cs.PrependReactor("get", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, node(notReadyTaint), nil
	})

	err := removeNotReadyTaint(func() (kubernetes.Interface, error) { return cs, nil })
	assert.Error(t, err)

	// The live node must be untouched: the not-ready taint is still present
	// because the conflicting patch was rejected.
	updated, getErr := cs.Tracker().Get(corev1.SchemeGroupVersion.WithResource("nodes"), "", testNodeName)
	assert.NoError(t, getErr)
	liveNode := updated.(*corev1.Node)
	assert.Equal(t, []corev1.Taint{notReadyTaint, otherTaint}, liveNode.Spec.Taints)
}

// TestRemoveTaintInBackground verifies the goroutine retries until the removal
// function succeeds.
func TestRemoveTaintInBackground(t *testing.T) {
	calls := 0
	removal := func(_ kubernetesClientGetter) error {
		calls++
		if calls == 3 {
			return nil
		}
		return fmt.Errorf("taint removal failed")
	}

	removeTaintInBackground(nil, removal)
	assert.Equal(t, 3, calls)
}
