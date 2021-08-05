/*
Copyright 2018 Intel Corporation

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
	. "github.com/onsi/ginkgo"
)

type test struct {
	text string
	body func(*TestContext)
}

var tests []test

// DescribeSanity must be used instead of the usual Ginkgo Describe to
// register a test block. The difference is that the body function
// will be called multiple times with the right context (when
// setting up a Ginkgo suite or a testing.T test, with the right
// configuration).
func DescribeSanity(text string, body func(*TestContext)) bool {
	tests = append(tests, test{text, body})
	return true
}

// registerTestsInGinkgo invokes the actual Gingko Describe
// for the tests registered earlier with DescribeSanity.
func registerTestsInGinkgo(sc *TestContext) {
	for _, test := range tests {
		Describe(test.text, func() {
			BeforeEach(func() {
				sc.Setup()
			})

			test.body(sc)

			AfterEach(func() {
				sc.Teardown()
			})
		})
	}
}
