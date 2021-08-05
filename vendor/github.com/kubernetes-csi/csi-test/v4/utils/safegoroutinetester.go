/*
Copyright 2017 Luis Pabón luis@portworx.com

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

package utils

import "fmt"

// SafeGoroutineTester is an implementation of the mock ... interface
// which can be used to use the mock functions in another go routine.
//
// The major issue is that the golang mock framework uses t.Fatalf()
// which causes a deadlock when called in another goroutine. To avoid
// this issue, this simple implementation prints the error then panics,
// which avoids the deadlock.
type SafeGoroutineTester struct{}

// Errorf prints the error to the screen then panics
func (s *SafeGoroutineTester) Errorf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
	panic("MOCK TEST ERROR")
}

// Fatalf prints the error to the screen then panics
func (s *SafeGoroutineTester) Fatalf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
	panic("MOCK TEST FATAL FAILURE")
}
