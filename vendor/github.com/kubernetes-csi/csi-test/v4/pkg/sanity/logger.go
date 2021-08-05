/*
Copyright 2020 Kubernetes Authors.

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

package sanity

import (
	"fmt"
	"log"

	. "github.com/onsi/ginkgo"
)

type logger struct {
	l         *log.Logger
	numFailed int
}

func newLogger(prefix string) *logger {
	return &logger{
		l: log.New(GinkgoWriter, prefix+" ", 0),
	}
}

// Infof logs a message without marking the test as failed.
func (l *logger) Infof(format string, v ...interface{}) {
	l.l.Printf(format, v...)
}

// Info logs a message without marking the test as failed.
func (l *logger) Info(v ...interface{}) {
	l.l.Print(v...)
}

// Errorf logs a message and marks the test as failed.
func (l *logger) Errorf(err error, format string, v ...interface{}) {
	if err == nil {
		return
	}
	l.numFailed++
	l.l.Printf(format, v...)
}

// ExpectNoErrors fails the spec if any error was logged.
func (l *logger) ExpectNoErrors(offset int) {
	if l.numFailed > 0 {
		Fail(fmt.Sprintf("recorded %d failure(s)", l.numFailed), offset+1)
	}
}
