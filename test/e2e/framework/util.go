package framework

import (
	"fmt"

	. "github.com/onsi/ginkgo"
)

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}
