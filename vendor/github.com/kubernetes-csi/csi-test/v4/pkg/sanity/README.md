# CSI Driver Sanity Tester

This library provides a simple way to ensure that a CSI driver conforms to
the CSI specification. There are two ways to leverage this testing framework.
For CSI drivers written in Golang, the framework provides a simple API function
to call to test the driver. Another way to run the test suite is to use the
command line program [csi-sanity](https://github.com/kubernetes-csi/csi-test/tree/master/cmd/csi-sanity).

## For Golang CSI Drivers
This framework leverages the Ginkgo BDD testing framework to deliver a descriptive
test suite for your driver. To test your driver, simply call the API in one of your
Golang `TestXXX` functions. For example:

```go
func TestMyDriver(t *testing.T) {
	// Setup the full driver and its environment
	... setup driver ...
	config := sanity.NewTestConfig()
	// Set configuration options as needed
	cfg.Address = endpoint

	// Now call the test suite
	sanity.Test(t, config)
}
```

Only one such test function is supported because under the hood a
Ginkgo test suite gets constructed and executed by the call.

Alternatively, the tests can also be embedded inside a Ginkgo test
suite. In that case it is possible to define multiple tests with
different configurations:

```go
var _ = Describe("MyCSIDriver", func () {
	Context("Config A", func () {
		var config &sanity.Config

		BeforeEach(func() {
			//... setup driver and config...
		})

		AfterEach(func() {
			//...tear down driver...
		})

		Describe("CSI sanity", func() {
			sanity.GinkgoTest(config)
		})
	})

	Context("Config B", func () {
		// other configs
	})
})
```

## Command line program
Please see [csi-sanity](https://github.com/kubernetes-csi/csi-test/tree/master/cmd/csi-sanity)
