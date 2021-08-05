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

package sanity

import (
	"context"
	"fmt"
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = DescribeSanity("Identity Service", func(sc *TestContext) {
	var (
		c csi.IdentityClient
	)

	BeforeEach(func() {
		c = csi.NewIdentityClient(sc.Conn)
	})

	Describe("GetPluginCapabilities", func() {
		It("should return appropriate capabilities", func() {
			req := &csi.GetPluginCapabilitiesRequest{}
			res, err := c.GetPluginCapabilities(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).NotTo(BeNil())

			By("checking successful response")
			for _, cap := range res.GetCapabilities() {
				switch cap.GetType().(type) {
				case *csi.PluginCapability_Service_:
					switch cap.GetService().GetType() {
					case csi.PluginCapability_Service_CONTROLLER_SERVICE:
					case csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS:
					default:
						Fail(fmt.Sprintf("Unknown service: %v\n", cap.GetService().GetType()))
					}
				case *csi.PluginCapability_VolumeExpansion_:
					switch cap.GetVolumeExpansion().GetType() {
					case csi.PluginCapability_VolumeExpansion_ONLINE:
					case csi.PluginCapability_VolumeExpansion_OFFLINE:
					default:
						Fail(fmt.Sprintf("Unknown volume expansion mode: %v\n", cap.GetVolumeExpansion().GetType()))
					}
				default:
					Fail(fmt.Sprintf("Unknown capability: %v\n", cap.GetType()))
				}
			}

		})

	})

	Describe("Probe", func() {
		It("should return appropriate information", func() {
			req := &csi.ProbeRequest{}
			res, err := c.Probe(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).NotTo(BeNil())

			By("verifying return status")
			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code() == codes.FailedPrecondition ||
				serverError.Code() == codes.OK).To(BeTrue())

			if res.GetReady() != nil {
				Expect(res.GetReady().GetValue() == true ||
					res.GetReady().GetValue() == false).To(BeTrue())
			}
		})
	})

	Describe("GetPluginInfo", func() {
		It("should return appropriate information", func() {
			req := &csi.GetPluginInfoRequest{}
			res, err := c.GetPluginInfo(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).NotTo(BeNil())

			By("verifying name size and characters")
			Expect(res.GetName()).ToNot(HaveLen(0))
			Expect(len(res.GetName())).To(BeNumerically("<=", 63))
			Expect(regexp.
				MustCompile("^[a-zA-Z][A-Za-z0-9-\\.\\_]{0,61}[a-zA-Z]$").
				MatchString(res.GetName())).To(BeTrue())
		})
	})
})
