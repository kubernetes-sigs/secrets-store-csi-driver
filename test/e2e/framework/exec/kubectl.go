/*
Copyright 2020 The Kubernetes Authors.

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

package exec

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"k8s.io/klog"
)

func execLocal(input io.Reader, cmd string, args ...string) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	klog.Infof("%s %s", cmd, strings.Join(args, " "))
	command := exec.Command(cmd, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if input != nil {
		command.Stdin = input
	}
	err := command.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func execWithInput(input []byte, cmd string, args ...string) (stdout []byte, stderr []byte, e error) {
	var r io.Reader
	if input != nil {
		r = bytes.NewReader(input)
	}
	return execLocal(r, cmd, args...)
}

func Kubectl(args ...string) ([]byte, []byte, error) {
	return execLocal(nil, "kubectl", args...)
}

func KubectlWithInput(input []byte, args ...string) ([]byte, []byte, error) {
	return execWithInput(input, "kubectl", args...)
}

func KubectlExec(kubeconfigPath, podName, namespace string, args ...string) ([]byte, []byte, error) {
	args = append([]string{
		"exec",
		fmt.Sprintf("--kubeconfig=%s", kubeconfigPath),
		fmt.Sprintf("--namespace=%s", namespace),
		podName,
		"--",
	}, args...)

	return Kubectl(args...)
}

func KubectlExecWithInput(input []byte, kubeconfigPath, podName, namespace string, args ...string) ([]byte, []byte, error) {
	args = append([]string{
		"exec",
		"-it",
		fmt.Sprintf("--kubeconfig=%s", kubeconfigPath),
		fmt.Sprintf("--namespace=%s", namespace),
		podName,
		"--",
	}, args...)

	return KubectlWithInput(input, args...)
}

func KubectlApply(kubeconfigPath, namespace string, file string) ([]byte, []byte, error) {
	return Kubectl(fmt.Sprintf("--kubeconfig=%s", kubeconfigPath),
		fmt.Sprintf("--namespace=%s", namespace),
		"apply",
		"-f",
		file,
	)
}
