package e2e

import (
	"os"
)

const (
	sleepTime = 1
)

const (
	Namespace      = "default"
	ContainerImage = "nginx"
	RoleName       = "example-role"
)

var (
	imageTag = os.Getenv("IMAGE_TAG")
)
