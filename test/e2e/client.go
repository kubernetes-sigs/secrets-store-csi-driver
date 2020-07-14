package e2e

import (
	"bytes"
	"io"
	"os/exec"
)

func execLocal(input io.Reader, cmd string, args ...string) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
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
