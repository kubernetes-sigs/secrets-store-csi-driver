/*
Copyright 2017 Kubernetes Authors.

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
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	// DefTestVolumeExpand defines the size increment for volume
	// expansion. It can be overriden by setting an
	// Config.TestVolumeExpandSize, which will be taken as absolute
	// value.
	DefTestExpandIncrement int64 = 1 * 1024 * 1024 * 1024

	MaxNameLength int = 128
)

func TestVolumeSize(sc *TestContext) int64 {
	return sc.Config.TestVolumeSize
}

func TestVolumeCapabilityWithAccessType(sc *TestContext, m csi.VolumeCapability_AccessMode_Mode) *csi.VolumeCapability {
	vc := &csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{Mode: m},
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{},
		},
	}
	if at := strings.TrimSpace(strings.ToLower(sc.Config.TestVolumeAccessType)); at == "block" {
		vc.AccessType = &csi.VolumeCapability_Block{
			Block: &csi.VolumeCapability_BlockVolume{},
		}
	}

	return vc
}

func TestVolumeExpandSize(sc *TestContext) int64 {
	if sc.Config.TestVolumeExpandSize > 0 {
		return sc.Config.TestVolumeExpandSize
	}
	return TestVolumeSize(sc) + DefTestExpandIncrement
}

func verifyVolumeInfo(v *csi.Volume) {
	Expect(v).NotTo(BeNil())
	Expect(v.GetVolumeId()).NotTo(BeEmpty())
}

func verifySnapshotInfo(snapshot *csi.Snapshot) {
	verifySnapshotInfoWithOffset(2, snapshot)
}

func verifySnapshotInfoWithOffset(offset int, snapshot *csi.Snapshot) {
	ExpectWithOffset(offset, snapshot).NotTo(BeNil())
	ExpectWithOffset(offset, snapshot.GetSnapshotId()).NotTo(BeEmpty())
	ExpectWithOffset(offset, snapshot.GetSourceVolumeId()).NotTo(BeEmpty())
	ExpectWithOffset(offset, snapshot.GetCreationTime()).NotTo(BeZero())
}

func isControllerCapabilitySupported(
	c csi.ControllerClient,
	capType csi.ControllerServiceCapability_RPC_Type,
) bool {

	caps, err := c.ControllerGetCapabilities(
		context.Background(),
		&csi.ControllerGetCapabilitiesRequest{})
	Expect(err).NotTo(HaveOccurred())
	Expect(caps).NotTo(BeNil())
	Expect(caps.GetCapabilities()).NotTo(BeNil())

	for _, cap := range caps.GetCapabilities() {
		Expect(cap.GetRpc()).NotTo(BeNil())
		if cap.GetRpc().GetType() == capType {
			return true
		}
	}
	return false
}

var _ = DescribeSanity("Controller Service [Controller Server]", func(sc *TestContext) {
	var r *Resources

	BeforeEach(func() {
		r = &Resources{
			Context:          sc,
			ControllerClient: csi.NewControllerClient(sc.ControllerConn),
			NodeClient:       csi.NewNodeClient(sc.Conn),
		}
	})

	AfterEach(func() {
		r.Cleanup()
	})

	Describe("ControllerGetCapabilities", func() {
		It("should return appropriate capabilities", func() {
			caps, err := r.ControllerGetCapabilities(
				context.Background(),
				&csi.ControllerGetCapabilitiesRequest{})

			By("checking successful response")
			Expect(err).NotTo(HaveOccurred())
			Expect(caps).NotTo(BeNil())
			Expect(caps.GetCapabilities()).NotTo(BeNil())

			for _, cap := range caps.GetCapabilities() {
				Expect(cap.GetRpc()).NotTo(BeNil())

				switch cap.GetRpc().GetType() {
				case csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME:
				case csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME:
				case csi.ControllerServiceCapability_RPC_LIST_VOLUMES:
				case csi.ControllerServiceCapability_RPC_GET_CAPACITY:
				case csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT:
				case csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS:
				case csi.ControllerServiceCapability_RPC_PUBLISH_READONLY:
				case csi.ControllerServiceCapability_RPC_CLONE_VOLUME:
				case csi.ControllerServiceCapability_RPC_EXPAND_VOLUME:
				case csi.ControllerServiceCapability_RPC_LIST_VOLUMES_PUBLISHED_NODES:
				case csi.ControllerServiceCapability_RPC_GET_VOLUME:
				case csi.ControllerServiceCapability_RPC_VOLUME_CONDITION:
				default:
					Fail(fmt.Sprintf("Unknown capability: %v\n", cap.GetRpc().GetType()))
				}
			}
		})
	})

	Describe("GetCapacity", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_GET_CAPACITY) {
				Skip("GetCapacity not supported")
			}
		})

		It("should return capacity (no optional values added)", func() {
			_, err := r.GetCapacity(
				context.Background(),
				&csi.GetCapacityRequest{})
			Expect(err).NotTo(HaveOccurred())

			// Since capacity is int64 we will not be checking it
			// The value of zero is a possible value.
		})
	})
	Describe("ListVolumes", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_LIST_VOLUMES) {
				Skip("ListVolumes not supported")
			}
		})

		It("should return appropriate values (no optional values added)", func() {
			vols, err := r.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())

			for _, vol := range vols.GetEntries() {
				verifyVolumeInfo(vol.GetVolume())
			}
		})

		It("should fail when an invalid starting_token is passed", func() {
			vols, err := r.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{
					StartingToken: "invalid-token",
				},
			)
			Expect(err).To(HaveOccurred())
			Expect(vols).To(BeNil())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.Aborted))
		})

		It("check the presence of new volumes and absence of deleted ones in the volume list", func() {
			// List Volumes before creating new volume.
			vols, err := r.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())

			totalVols := len(vols.GetEntries())

			By("creating a volume")
			name := "sanity"

			// Create a new volume.
			req := &csi.CreateVolumeRequest{
				Name: name,
				VolumeCapabilities: []*csi.VolumeCapability{
					TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
				},
				Secrets:    sc.Secrets.CreateVolumeSecret,
				Parameters: sc.Config.TestVolumeParameters,
			}

			vol := r.MustCreateVolume(context.Background(), req)

			// List volumes and check for the newly created volume.
			vols, err = r.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())
			Expect(len(vols.GetEntries())).To(Equal(totalVols + 1))

			By("deleting the volume")

			delReq := &csi.DeleteVolumeRequest{
				VolumeId: vol.GetVolume().GetVolumeId(),
				Secrets:  sc.Secrets.DeleteVolumeSecret,
			}

			_, err = r.DeleteVolume(context.Background(), delReq)
			Expect(err).NotTo(HaveOccurred())

			// List volumes and check if the deleted volume exists in the volume list.
			vols, err = r.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())
			Expect(len(vols.GetEntries())).To(Equal(totalVols))
		})

		// Disabling this below case as it is fragile and results are inconsistent
		// when no of volumes are different. The test might fail on a driver
		// which implements the pagination based on index just by altering
		// minVolCount := 4 and maxEntries := 3
		// Related discussion links:
		//  https://github.com/intel/pmem-csi/pull/424#issuecomment-540499938
		//  https://github.com/kubernetes-csi/csi-test/issues/223
		XIt("pagination should detect volumes added between pages and accept tokens when the last volume from a page is deleted", func() {
			// minVolCount is the minimum number of volumes expected to exist,
			// based on which paginated volume listing is performed.
			minVolCount := 3
			// maxEntries is the maximum entries in list volume request.
			maxEntries := 2
			// existing_vols to keep a record of the volumes that should exist
			existing_vols := map[string]bool{}

			// Get the number of existing volumes.
			vols, err := r.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())

			initialTotalVols := len(vols.GetEntries())

			for _, vol := range vols.GetEntries() {
				existing_vols[vol.Volume.VolumeId] = true
			}

			if minVolCount <= initialTotalVols {
				minVolCount = initialTotalVols
			} else {
				// Ensure minimum minVolCount volumes exist.
				By("creating required new volumes")
				for i := initialTotalVols; i < minVolCount; i++ {
					name := "sanity" + strconv.Itoa(i)
					req := &csi.CreateVolumeRequest{
						Name: name,
						VolumeCapabilities: []*csi.VolumeCapability{
							TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
						},
						Secrets:    sc.Secrets.CreateVolumeSecret,
						Parameters: sc.Config.TestVolumeParameters,
					}

					vol, err := r.CreateVolume(context.Background(), req)
					Expect(err).NotTo(HaveOccurred())
					Expect(vol).NotTo(BeNil())
					// Register the volume so it's automatically cleaned
					existing_vols[vol.Volume.VolumeId] = true
				}
			}

			// Request list volumes with max entries maxEntries.
			vols, err = r.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{
					MaxEntries: int32(maxEntries),
				})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())
			Expect(len(vols.GetEntries())).To(Equal(maxEntries))

			nextToken := vols.GetNextToken()

			By("removing all listed volumes")
			for _, vol := range vols.GetEntries() {
				Expect(existing_vols[vol.Volume.VolumeId]).To(BeTrue())
				delReq := &csi.DeleteVolumeRequest{
					VolumeId: vol.Volume.VolumeId,
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				}

				_, err := r.DeleteVolume(context.Background(), delReq)
				Expect(err).NotTo(HaveOccurred())
				vol_id := vol.Volume.VolumeId
				existing_vols[vol_id] = false
			}

			By("creating a new volume")
			req := &csi.CreateVolumeRequest{
				Name: "new-addition",
				VolumeCapabilities: []*csi.VolumeCapability{
					TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
				},
				Secrets:    sc.Secrets.CreateVolumeSecret,
				Parameters: sc.Config.TestVolumeParameters,
			}
			vol := r.MustCreateVolume(context.Background(), req)
			existing_vols[vol.Volume.VolumeId] = true

			vols, err = r.ListVolumes(
				context.Background(),
				&csi.ListVolumesRequest{
					StartingToken: nextToken,
				})
			Expect(err).NotTo(HaveOccurred())
			Expect(vols).NotTo(BeNil())
			expected_num_volumes := minVolCount - maxEntries + 1
			// Depending on the plugin implementation we may be missing volumes, but should not get duplicates
			Expect(len(vols.GetEntries()) <= expected_num_volumes).To(BeTrue())
			for _, vol := range vols.GetEntries() {
				Expect(existing_vols[vol.Volume.VolumeId]).To(BeTrue())
				existing_vols[vol.Volume.VolumeId] = false
			}
		})
	})

	Describe("CreateVolume", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME) {
				Skip("CreateVolume not supported")
			}
		})

		It("should fail when no name is provided", func() {
			_, err := r.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail when no volume capabilities are provided", func() {
			name := UniqueString("sanity-controller-create-no-volume-capabilities")
			_, err := r.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name:       name,
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		// TODO: whether CreateVolume request with no capacity should fail or not depends on driver implementation
		It("should return appropriate values SingleNodeWriter NoCapacity", func() {

			By("creating a volume")
			name := UniqueString("sanity-controller-create-single-no-capacity")

			r.MustCreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: TestVolumeSize(sc),
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
		})

		It("should return appropriate values SingleNodeWriter WithCapacity 1Gi", func() {

			By("creating a volume")
			name := UniqueString("sanity-controller-create-single-with-capacity")

			vol, err := r.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: TestVolumeSize(sc),
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			if serverError, ok := status.FromError(err); ok &&
				(serverError.Code() == codes.OutOfRange || serverError.Code() == codes.Unimplemented) {
				Skip("Required bytes not supported")
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(vol).NotTo(BeNil())
			Expect(vol.GetVolume()).NotTo(BeNil())
			Expect(vol.GetVolume().GetVolumeId()).NotTo(BeEmpty())
			Expect(vol.GetVolume().GetCapacityBytes()).To(Or(BeNumerically(">=", TestVolumeSize(sc)), BeZero()))
		})

		It("should not fail when requesting to create a volume with already existing name and same capacity", func() {

			By("creating a volume")
			name := UniqueString("sanity-controller-create-twice")
			size := TestVolumeSize(sc)

			vol1 := r.MustCreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: size,
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)

			Expect(vol1.GetVolume().GetCapacityBytes()).To(Or(BeNumerically(">=", size), BeZero()))

			vol2 := r.MustCreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: size,
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(vol2.GetVolume().GetCapacityBytes()).To(Or(BeNumerically(">=", size), BeZero()))
			Expect(vol1.GetVolume().GetVolumeId()).To(Equal(vol2.GetVolume().GetVolumeId()))
		})

		It("should fail when requesting to create a volume with already existing name and different capacity", func() {

			By("creating a volume")
			name := UniqueString("sanity-controller-create-twice-different")
			size1 := TestVolumeSize(sc)

			r.MustCreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: size1,
						LimitBytes:    size1,
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			size2 := 2 * TestVolumeSize(sc)

			_, err := r.CreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: size2,
						LimitBytes:    size2,
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(err).To(HaveOccurred())
			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.AlreadyExists))
		})

		It("should not fail when creating volume with maximum-length name", func() {

			nameBytes := make([]byte, MaxNameLength)
			for i := 0; i < MaxNameLength; i++ {
				nameBytes[i] = 'a'
			}
			name := string(nameBytes)
			By("creating a volume")
			size := TestVolumeSize(sc)

			vol := r.MustCreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: size,
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)
			Expect(vol.GetVolume().GetCapacityBytes()).To(Or(BeNumerically(">=", size), BeZero()))
		})

		It("should create volume from an existing source snapshot", func() {
			if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT) {
				Skip("Snapshot not supported")
			}

			By("creating a snapshot")
			vol1Req := MakeCreateVolumeReq(sc, UniqueString("sanity-controller-source-vol"))
			snap, _ := r.MustCreateSnapshotFromVolumeRequest(context.Background(), vol1Req, UniqueString("sanity-controller-snap-from-vol"))

			By("creating a volume from source snapshot")
			vol2Name := UniqueString("sanity-controller-vol-from-snap")
			vol2Req := MakeCreateVolumeReq(sc, vol2Name)
			vol2Req.VolumeContentSource = &csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Snapshot{
					Snapshot: &csi.VolumeContentSource_SnapshotSource{
						SnapshotId: snap.GetSnapshot().GetSnapshotId(),
					},
				},
			}
			_, err := r.CreateVolume(context.Background(), vol2Req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail when the volume source snapshot is not found", func() {
			if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT) {
				Skip("Snapshot not supported")
			}

			By("creating a volume from source snapshot")
			volName := UniqueString("sanity-controller-vol-from-snap")
			volReq := MakeCreateVolumeReq(sc, volName)
			volReq.VolumeContentSource = &csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Snapshot{
					Snapshot: &csi.VolumeContentSource_SnapshotSource{
						SnapshotId: "non-existing-snapshot-id",
					},
				},
			}
			_, err := r.CreateVolume(context.Background(), volReq)
			Expect(err).To(HaveOccurred())
			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.NotFound))
		})

		It("should create volume from an existing source volume", func() {
			if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_CLONE_VOLUME) {
				Skip("Volume Cloning not supported")
			}

			By("creating a volume")
			vol1Name := UniqueString("sanity-controller-source-vol")
			vol1Req := MakeCreateVolumeReq(sc, vol1Name)
			volume1 := r.MustCreateVolume(context.Background(), vol1Req)

			By("creating a volume from source volume")
			vol2Name := UniqueString("sanity-controller-vol-from-vol")
			vol2Req := MakeCreateVolumeReq(sc, vol2Name)
			vol2Req.VolumeContentSource = &csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Volume{
					Volume: &csi.VolumeContentSource_VolumeSource{
						VolumeId: volume1.GetVolume().GetVolumeId(),
					},
				},
			}
			_, err := r.CreateVolume(context.Background(), vol2Req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail when the volume source volume is not found", func() {
			if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_CLONE_VOLUME) {
				Skip("Volume Cloning not supported")
			}

			By("creating a volume from source snapshot")
			volName := UniqueString("sanity-controller-vol-from-snap")
			volReq := MakeCreateVolumeReq(sc, volName)
			volReq.VolumeContentSource = &csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Volume{
					Volume: &csi.VolumeContentSource_VolumeSource{
						VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					},
				},
			}
			_, err := r.CreateVolume(context.Background(), volReq)
			Expect(err).To(HaveOccurred())
			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.NotFound))
		})
	})

	Describe("DeleteVolume", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME) {
				Skip("DeleteVolume not supported")
			}
		})

		It("should fail when no volume id is provided", func() {

			_, err := r.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					Secrets: sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should succeed when an invalid volume id is used", func() {

			_, err := r.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: sc.Config.IDGen.GenerateInvalidVolumeID(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return appropriate values (no optional values added)", func() {

			// Create Volume First
			By("creating a volume")
			name := UniqueString("sanity-controller-create-appropriate")

			vol := r.MustCreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: TestVolumeSize(sc),
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)

			// Delete Volume
			By("deleting a volume")

			_, err := r.DeleteVolume(
				context.Background(),
				&csi.DeleteVolumeRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					Secrets:  sc.Secrets.DeleteVolumeSecret,
				},
			)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("ValidateVolumeCapabilities", func() {
		It("should fail when no volume id is provided", func() {

			_, err := r.ValidateVolumeCapabilities(
				context.Background(),
				&csi.ValidateVolumeCapabilitiesRequest{
					Secrets: sc.Secrets.ControllerValidateVolumeCapabilitiesSecret,
				})
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail when no volume capabilities are provided", func() {

			// Create Volume First
			By("creating a single node writer volume")
			name := UniqueString("sanity-controller-validate-nocaps")

			vol := r.MustCreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: TestVolumeSize(sc),
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)

			_, err := r.ValidateVolumeCapabilities(
				context.Background(),
				&csi.ValidateVolumeCapabilitiesRequest{
					VolumeId:           vol.GetVolume().GetVolumeId(),
					VolumeCapabilities: nil,
					Secrets:            sc.Secrets.ControllerValidateVolumeCapabilitiesSecret,
				})
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should return appropriate values (no optional values added)", func() {

			// Create Volume First
			By("creating a single node writer volume")
			name := UniqueString("sanity-controller-validate")

			vol := r.MustCreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: TestVolumeSize(sc),
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)

			// ValidateVolumeCapabilities
			By("validating volume capabilities")
			valivolcap, err := r.ValidateVolumeCapabilities(
				context.Background(),
				&csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: vol.GetVolume().GetVolumeId(),
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					Secrets: sc.Secrets.ControllerValidateVolumeCapabilitiesSecret,
				})
			Expect(err).NotTo(HaveOccurred())
			Expect(valivolcap).NotTo(BeNil())

			// If confirmation is provided then it is REQUIRED to provide
			// the volume capabilities
			if valivolcap.GetConfirmed() != nil {
				Expect(valivolcap.GetConfirmed().GetVolumeCapabilities()).NotTo(BeEmpty())
			}
		})

		It("should fail when the requested volume does not exist", func() {

			_, err := r.ValidateVolumeCapabilities(
				context.Background(),
				&csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					Secrets: sc.Secrets.ControllerValidateVolumeCapabilitiesSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.NotFound))
		})
	})

	Describe("ControllerPublishVolume", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME) {
				Skip("ControllerPublishVolume not supported")
			}
		})

		It("should fail when no volume id is provided", func() {

			_, err := r.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					Secrets: sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail when no node id is provided", func() {

			_, err := r.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail when no volume capability is provided", func() {

			_, err := r.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					NodeId:   sc.Config.IDGen.GenerateUniqueValidNodeID(),
					Secrets:  sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})

		It("should fail when publishing more volumes than the node max attach limit", func() {
			if !sc.Config.TestNodeVolumeAttachLimit {
				Skip("testnodevolumeattachlimit not enabled")
			}

			By("getting node info")
			nodeInfo, err := r.NodeGetInfo(
				context.Background(),
				&csi.NodeGetInfoRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(nodeInfo).NotTo(BeNil())

			if nodeInfo.MaxVolumesPerNode <= 0 {
				Skip("No MaxVolumesPerNode")
			}

			nid := nodeInfo.GetNodeId()
			Expect(nid).NotTo(BeEmpty())

			By("publishing volumes")
			for i := int64(0); i < nodeInfo.MaxVolumesPerNode; i++ {
				name := UniqueString(fmt.Sprintf("sanity-max-attach-limit-vol-%d", i))
				vol := r.MustCreateVolume(context.Background(), MakeCreateVolumeReq(sc, name))
				volID := vol.GetVolume().GetVolumeId()
				r.MustControllerPublishVolume(
					context.Background(),
					MakeControllerPublishVolumeReq(sc, volID, nid),
				)
			}

			extraVolName := UniqueString("sanity-max-attach-limit-vol+1")
			vol := r.MustCreateVolume(context.Background(), MakeCreateVolumeReq(sc, extraVolName))

			_, err = r.ControllerPublishVolume(
				context.Background(),
				MakeControllerPublishVolumeReq(sc, vol.Volume.VolumeId, nid),
			)
			Expect(err).To(HaveOccurred())
		})

		It("should fail when the volume does not exist", func() {

			By("calling controller publish on a non-existent volume")

			conpubvol, err := r.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId:         sc.Config.IDGen.GenerateUniqueValidVolumeID(),
					NodeId:           sc.Config.IDGen.GenerateUniqueValidNodeID(),
					VolumeCapability: TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					Readonly:         false,
					Secrets:          sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())
			Expect(conpubvol).To(BeNil())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.NotFound))
		})

		It("should fail when the node does not exist", func() {

			// Create Volume First
			By("creating a single node writer volume")
			name := UniqueString("sanity-controller-wrong-node")

			vol := r.MustCreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)

			// ControllerPublishVolume
			By("calling controllerpublish on that volume")

			conpubvol, err := r.ControllerPublishVolume(
				context.Background(),
				&csi.ControllerPublishVolumeRequest{
					VolumeId:         vol.GetVolume().GetVolumeId(),
					NodeId:           sc.Config.IDGen.GenerateUniqueValidNodeID(),
					VolumeCapability: TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					Readonly:         false,
					Secrets:          sc.Secrets.ControllerPublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())
			Expect(conpubvol).To(BeNil())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.NotFound))
		})

		It("should fail when the volume is already published but is incompatible", func() {
			if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_PUBLISH_READONLY) {
				Skip("ControllerPublishVolume.readonly field not supported")
			}

			// Create Volume First
			By("creating a single node writer volume")
			name := UniqueString("sanity-controller-published-incompatible")

			vol := r.MustCreateVolume(
				context.Background(),
				&csi.CreateVolumeRequest{
					Name: name,
					VolumeCapabilities: []*csi.VolumeCapability{
						TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
					},
					Secrets:    sc.Secrets.CreateVolumeSecret,
					Parameters: sc.Config.TestVolumeParameters,
				},
			)

			By("getting a node id")
			nid, err := r.NodeGetInfo(
				context.Background(),
				&csi.NodeGetInfoRequest{})
			Expect(err).NotTo(HaveOccurred())
			Expect(nid).NotTo(BeNil())
			Expect(nid.GetNodeId()).NotTo(BeEmpty())

			// ControllerPublishVolume
			By("calling controllerpublish on that volume")

			pubReq := &csi.ControllerPublishVolumeRequest{
				VolumeId:         vol.GetVolume().GetVolumeId(),
				NodeId:           nid.GetNodeId(),
				VolumeCapability: TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
				Readonly:         false,
				Secrets:          sc.Secrets.ControllerPublishVolumeSecret,
			}

			conpubvol := r.MustControllerPublishVolume(context.Background(), pubReq)

			// Publish again with different attributes.
			pubReq.Readonly = true

			conpubvol, err = r.ControllerPublishVolume(context.Background(), pubReq)
			Expect(err).To(HaveOccurred())
			Expect(conpubvol).To(BeNil())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.AlreadyExists))
		})
	})

	Describe("volume lifecycle", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME) {
				Skip("Controller Publish, UnpublishVolume not supported")
			}
		})

		It("should work", func() {
			VolumeLifecycle(r, sc, 1)
		})

		It("should be idempotent", func() {
			VolumeLifecycle(r, sc, sc.Config.IdempotentCount)
		})
	})

	Describe("ControllerUnpublishVolume", func() {
		BeforeEach(func() {
			if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME) {
				Skip("ControllerUnpublishVolume not supported")
			}
		})

		It("should fail when no volume id is provided", func() {

			_, err := r.ControllerUnpublishVolume(
				context.Background(),
				&csi.ControllerUnpublishVolumeRequest{
					Secrets: sc.Secrets.ControllerUnpublishVolumeSecret,
				},
			)
			Expect(err).To(HaveOccurred())

			serverError, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
		})
	})
})

var _ = DescribeSanity("ListSnapshots [Controller Server]", func(sc *TestContext) {
	var r *Resources

	BeforeEach(func() {
		r = &Resources{
			Context:          sc,
			ControllerClient: csi.NewControllerClient(sc.ControllerConn),
			NodeClient:       csi.NewNodeClient(sc.Conn),
		}

		if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS) {
			Skip("ListSnapshots not supported")
		}
	})

	AfterEach(func() {
		r.Cleanup()
	})

	It("should return appropriate values (no optional values added)", func() {
		snapshots, err := r.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		for _, snapshot := range snapshots.GetEntries() {
			verifySnapshotInfo(snapshot.GetSnapshot())
		}
	})

	It("should return snapshots that match the specified snapshot id", func() {
		// The test creates three snapshots: one that we intend to find by
		// snapshot ID, and two unrelated ones that must not be returned by
		// ListSnapshots.

		By("creating first unrelated snapshot")
		// Create volume source and afterwards the first unrelated snapshot.
		volReq := MakeCreateVolumeReq(sc, "listSnapshots-volume-unrelated1")
		r.MustCreateSnapshotFromVolumeRequest(context.Background(), volReq, "listSnapshots-snapshot-unrelated1")

		By("creating target snapshot")
		// Create volume source and afterwards the target snapshot.
		volReq = MakeCreateVolumeReq(sc, "listSnapshots-volume-target")
		snapshotTarget, _ := r.MustCreateSnapshotFromVolumeRequest(context.Background(), volReq, "listSnapshots-snapshot-target")

		By("creating second unrelated snapshot")
		// Create volume source and afterwards the second unrelated snapshot.
		volReq = MakeCreateVolumeReq(sc, "listSnapshots-volume-unrelated2")
		r.MustCreateSnapshotFromVolumeRequest(context.Background(), volReq, "listSnapshots-snapshot-unrelated2")

		By("listing snapshots")
		snapshots, err := r.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SnapshotId: snapshotTarget.GetSnapshot().GetSnapshotId()})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(snapshots.GetEntries()).To(HaveLen(1))
		verifySnapshotInfo(snapshots.GetEntries()[0].GetSnapshot())
		Expect(snapshots.GetEntries()[0].GetSnapshot().GetSnapshotId()).To(Equal(snapshotTarget.GetSnapshot().GetSnapshotId()))
	})

	It("should return empty when the specified snapshot id does not exist", func() {

		snapshots, err := r.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SnapshotId: "none-exist-id"})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(snapshots.GetEntries()).To(BeEmpty())
	})

	It("should return snapshots that match the specified source volume id", func() {

		// The test creates three snapshots: one that we intend to find by
		// source volume ID, and two unrelated ones that must not be returned by
		// ListSnapshots.

		By("creating first unrelated snapshot")
		// Create volume source and afterwards the first unrelated snapshot.
		volReq := MakeCreateVolumeReq(sc, "listSnapshots-volume-unrelated1")
		r.MustCreateSnapshotFromVolumeRequest(context.Background(), volReq, "listSnapshots-snapshot-unrelated1")

		By("creating target snapshot")
		// Create volume source and afterwards the target snapshot.
		volReq = MakeCreateVolumeReq(sc, "listSnapshots-volume-target")
		snapshotTarget, _ := r.MustCreateSnapshotFromVolumeRequest(context.Background(), volReq, "listSnapshots-snapshot-target")

		By("creating second unrelated snapshot")
		// Create volume source and afterwards the second unrelated snapshot.
		volReq = MakeCreateVolumeReq(sc, "listSnapshots-volume-unrelated2")
		r.MustCreateSnapshotFromVolumeRequest(context.Background(), volReq, "listSnapshots-snapshot-unrelated2")

		By("listing snapshots")
		snapshots, err := r.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SourceVolumeId: snapshotTarget.GetSnapshot().GetSourceVolumeId()})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(snapshots.GetEntries()).To(HaveLen(1))
		snapshot := snapshots.GetEntries()[0].GetSnapshot()
		verifySnapshotInfo(snapshot)
		Expect(snapshot.GetSourceVolumeId()).To(Equal(snapshotTarget.GetSnapshot().GetSourceVolumeId()))
	})

	It("should return empty when the specified source volume id does not exist", func() {

		snapshots, err := r.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{SourceVolumeId: sc.Config.IDGen.GenerateUniqueValidVolumeID()})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(snapshots.GetEntries()).To(BeEmpty())
	})

	It("check the presence of new snapshots in the snapshot list", func() {
		// List Snapshots before creating new snapshots.
		snapshots, err := r.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		totalSnapshots := len(snapshots.GetEntries())

		By("creating a snapshot")
		volReq := MakeCreateVolumeReq(sc, "listSnapshots-volume-3")
		snapshot, _ := r.MustCreateSnapshotFromVolumeRequest(context.Background(), volReq, "listSnapshots-snapshot-3")
		verifySnapshotInfo(snapshot.GetSnapshot())

		snapshots, err = r.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(snapshots.GetEntries()).To(HaveLen(totalSnapshots + 1))

		By("deleting the snapshot")
		_, err = r.DeleteSnapshot(context.Background(), &csi.DeleteSnapshotRequest{SnapshotId: snapshot.Snapshot.SnapshotId})
		Expect(err).NotTo(HaveOccurred())

		By("checking if deleted snapshot is omitted")
		snapshots, err = r.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())
		Expect(snapshots.GetEntries()).To(HaveLen(totalSnapshots))
	})

	It("should return next token when a limited number of entries are requested", func() {
		// minSnapshotCount is the minimum number of snapshots expected to exist,
		// based on which paginated snapshot listing is performed.
		minSnapshotCount := 5
		// maxEntried is the maximum entries in list snapshot request.
		maxEntries := 2
		// currentTotalVols is the total number of volumes at a given time. It
		// is used to verify that all the snapshots have been listed.
		currentTotalSnapshots := 0

		// Get the number of existing volumes.
		snapshots, err := r.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		initialTotalSnapshots := len(snapshots.GetEntries())
		currentTotalSnapshots = initialTotalSnapshots

		// Ensure minimum minVolCount volumes exist.
		if initialTotalSnapshots < minSnapshotCount {

			By("creating required new volumes")
			requiredSnapshots := minSnapshotCount - initialTotalSnapshots

			for i := 1; i <= requiredSnapshots; i++ {
				volReq := MakeCreateVolumeReq(sc, "volume"+strconv.Itoa(i))
				snapshot, _ := r.MustCreateSnapshotFromVolumeRequest(context.Background(), volReq, "snapshot"+strconv.Itoa(i))
				verifySnapshotInfo(snapshot.GetSnapshot())
			}

			// Update the current total snapshots count.
			currentTotalSnapshots += requiredSnapshots
		}

		// Request list snapshots with max entries maxEntries.
		snapshots, err = r.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{
				MaxEntries: int32(maxEntries),
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		nextToken := snapshots.GetNextToken()

		Expect(snapshots.GetEntries()).To(HaveLen(maxEntries))

		// Request list snapshots with starting_token and no max entries.
		snapshots, err = r.ListSnapshots(
			context.Background(),
			&csi.ListSnapshotsRequest{
				StartingToken: nextToken,
			})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshots).NotTo(BeNil())

		// Ensure that all the remaining entries are returned at once.
		Expect(snapshots.GetEntries()).To(HaveLen(currentTotalSnapshots - maxEntries))
	})
})

var _ = DescribeSanity("DeleteSnapshot [Controller Server]", func(sc *TestContext) {
	var r *Resources

	BeforeEach(func() {
		r = &Resources{
			Context:          sc,
			ControllerClient: csi.NewControllerClient(sc.ControllerConn),
			NodeClient:       csi.NewNodeClient(sc.Conn),
		}

		if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT) {
			Skip("DeleteSnapshot not supported")
		}
	})

	AfterEach(func() {
		r.Cleanup()
	})

	It("should fail when no snapshot id is provided", func() {

		req := &csi.DeleteSnapshotRequest{}

		if sc.Secrets != nil {
			req.Secrets = sc.Secrets.DeleteSnapshotSecret
		}

		_, err := r.DeleteSnapshot(context.Background(), req)
		Expect(err).To(HaveOccurred())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should succeed when an invalid snapshot id is used", func() {

		req := MakeDeleteSnapshotReq(sc, "reallyfakesnapshotid")
		_, err := r.DeleteSnapshot(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return appropriate values (no optional values added)", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "DeleteSnapshot-volume-1")
		volume, err := r.CreateVolume(context.Background(), volReq)
		Expect(err).NotTo(HaveOccurred())

		// Create Snapshot First
		By("creating a snapshot")
		snapshotReq := MakeCreateSnapshotReq(sc, "DeleteSnapshot-snapshot-1", volume.GetVolume().GetVolumeId())
		r.MustCreateSnapshot(context.Background(), snapshotReq)
	})
})

var _ = DescribeSanity("CreateSnapshot [Controller Server]", func(sc *TestContext) {
	var r *Resources

	BeforeEach(func() {
		r = &Resources{
			Context:          sc,
			ControllerClient: csi.NewControllerClient(sc.ControllerConn),
			NodeClient:       csi.NewNodeClient(sc.Conn),
		}

		if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT) {
			Skip("CreateSnapshot not supported")
		}
	})

	AfterEach(func() {
		r.Cleanup()
	})

	It("should fail when no name is provided", func() {

		req := &csi.CreateSnapshotRequest{
			SourceVolumeId: "testId",
		}

		if sc.Secrets != nil {
			req.Secrets = sc.Secrets.CreateSnapshotSecret
		}

		_, err := r.CreateSnapshot(context.Background(), req)
		Expect(err).To(HaveOccurred())
		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail when no source volume id is provided", func() {

		req := &csi.CreateSnapshotRequest{
			Name: "name",
		}

		if sc.Secrets != nil {
			req.Secrets = sc.Secrets.CreateSnapshotSecret
		}

		_, err := r.CreateSnapshot(context.Background(), req)
		Expect(err).To(HaveOccurred())
		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should succeed when requesting to create a snapshot with already existing name and same source volume ID", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "CreateSnapshot-volume-1")
		volume := r.MustCreateVolume(context.Background(), volReq)

		By("creating a snapshot")
		snapReq1 := MakeCreateSnapshotReq(sc, "CreateSnapshot-snapshot-1", volume.GetVolume().GetVolumeId())
		r.MustCreateSnapshot(context.Background(), snapReq1)

		By("creating a snapshot with the same name and source volume ID")
		r.MustCreateSnapshot(context.Background(), snapReq1)
	})

	It("should fail when requesting to create a snapshot with already existing name and different source volume ID", func() {

		By("creating a snapshot")
		volReq := MakeCreateVolumeReq(sc, "CreateSnapshot-volume-2")
		r.MustCreateSnapshotFromVolumeRequest(context.Background(), volReq, "CreateSnapshot-snapshot-2")

		By("creating a new source volume")
		volReq = MakeCreateVolumeReq(sc, "CreateSnapshot-volume-3")
		volume2 := r.MustCreateVolume(context.Background(), volReq)

		By("creating a snapshot with the same name but different source volume ID")
		req := MakeCreateSnapshotReq(sc, "CreateSnapshot-snapshot-2", volume2.GetVolume().GetVolumeId())
		_, err := r.CreateSnapshot(context.Background(), req)
		Expect(err).To(HaveOccurred())
		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.AlreadyExists))
	})

	It("should succeed when creating snapshot with maximum-length name", func() {

		By("creating a volume")
		volReq := MakeCreateVolumeReq(sc, "CreateSnapshot-volume-3")
		volume := r.MustCreateVolume(context.Background(), volReq)

		nameBytes := make([]byte, MaxNameLength)
		for i := 0; i < MaxNameLength; i++ {
			nameBytes[i] = 'a'
		}
		name := string(nameBytes)

		By("creating a snapshot")
		snapReq1 := MakeCreateSnapshotReq(sc, name, volume.GetVolume().GetVolumeId())
		r.MustCreateSnapshot(context.Background(), snapReq1)

		// TODO: review if the second snapshot create is really necessary
		r.MustCreateSnapshot(context.Background(), snapReq1)
	})
})

var _ = DescribeSanity("ExpandVolume [Controller Server]", func(sc *TestContext) {
	var r *Resources

	BeforeEach(func() {
		r = &Resources{
			ControllerClient: csi.NewControllerClient(sc.ControllerConn),
			Context:          sc,
		}

		if !isControllerCapabilitySupported(r, csi.ControllerServiceCapability_RPC_EXPAND_VOLUME) {
			Skip("ControllerExpandVolume not supported")
		}
	})

	AfterEach(func() {
		r.Cleanup()
	})

	It("should fail if no volume id is given", func() {
		expReq := &csi.ControllerExpandVolumeRequest{
			VolumeId: "",
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: TestVolumeExpandSize(sc),
			},
			Secrets: sc.Secrets.ControllerExpandVolumeSecret,
		}
		rsp, err := r.ControllerExpandVolume(context.Background(), expReq)
		Expect(err).To(HaveOccurred())
		Expect(rsp).To(BeNil())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should fail if no capacity range is given", func() {
		expReq := &csi.ControllerExpandVolumeRequest{
			VolumeId: "",
			Secrets:  sc.Secrets.ControllerExpandVolumeSecret,
		}
		rsp, err := r.ControllerExpandVolume(context.Background(), expReq)
		Expect(err).To(HaveOccurred())
		Expect(rsp).To(BeNil())

		serverError, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(serverError.Code()).To(Equal(codes.InvalidArgument))
	})

	It("should work", func() {

		By("creating a new volume")
		name := UniqueString("sanity-expand-volume")

		// Create a new volume.
		req := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
			},
			Parameters: sc.Config.TestVolumeParameters,
			Secrets:    sc.Secrets.CreateVolumeSecret,
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: TestVolumeSize(sc),
			},
		}
		vol := r.MustCreateVolume(context.Background(), req)

		By("expanding the volume")
		expReq := &csi.ControllerExpandVolumeRequest{
			VolumeId: vol.GetVolume().GetVolumeId(),
			CapacityRange: &csi.CapacityRange{
				RequiredBytes: TestVolumeExpandSize(sc),
			},
			Secrets:          sc.Secrets.ControllerExpandVolumeSecret,
			VolumeCapability: TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
		}
		rsp, err := r.ControllerExpandVolume(context.Background(), expReq)
		Expect(err).NotTo(HaveOccurred())
		Expect(rsp).NotTo(BeNil())
		Expect(rsp.GetCapacityBytes()).To(Equal(TestVolumeExpandSize(sc)))
	})
})

func MakeCreateVolumeReq(sc *TestContext, name string) *csi.CreateVolumeRequest {
	size1 := TestVolumeSize(sc)

	req := &csi.CreateVolumeRequest{
		Name: name,
		VolumeCapabilities: []*csi.VolumeCapability{
			TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
		},
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: size1,
			LimitBytes:    size1,
		},
		Parameters: sc.Config.TestVolumeParameters,
	}

	if sc.Secrets != nil {
		req.Secrets = sc.Secrets.CreateVolumeSecret
	}

	return req
}

func MakeCreateSnapshotReq(sc *TestContext, name, sourceVolumeId string) *csi.CreateSnapshotRequest {
	req := &csi.CreateSnapshotRequest{
		Name:           name,
		SourceVolumeId: sourceVolumeId,
		Parameters:     sc.Config.TestSnapshotParameters,
	}

	if sc.Secrets != nil {
		req.Secrets = sc.Secrets.CreateSnapshotSecret
	}

	return req
}

func MakeDeleteSnapshotReq(sc *TestContext, id string) *csi.DeleteSnapshotRequest {
	delSnapReq := &csi.DeleteSnapshotRequest{
		SnapshotId: id,
	}

	if sc.Secrets != nil {
		delSnapReq.Secrets = sc.Secrets.DeleteSnapshotSecret
	}

	return delSnapReq
}

func MakeDeleteVolumeReq(sc *TestContext, id string) *csi.DeleteVolumeRequest {
	delVolReq := &csi.DeleteVolumeRequest{
		VolumeId: id,
	}

	if sc.Secrets != nil {
		delVolReq.Secrets = sc.Secrets.DeleteVolumeSecret
	}

	return delVolReq
}

// MakeControllerPublishVolumeReq creates and returns a ControllerPublishVolumeRequest.
func MakeControllerPublishVolumeReq(sc *TestContext, volID, nodeID string) *csi.ControllerPublishVolumeRequest {
	return &csi.ControllerPublishVolumeRequest{
		VolumeId:         volID,
		NodeId:           nodeID,
		VolumeCapability: TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
		Readonly:         false,
		Secrets:          sc.Secrets.ControllerPublishVolumeSecret,
	}
}

// MakeControllerUnpublishVolumeReq creates and returns a ControllerUnpublishVolumeRequest.
func MakeControllerUnpublishVolumeReq(sc *TestContext, volID, nodeID string) *csi.ControllerUnpublishVolumeRequest {
	return &csi.ControllerUnpublishVolumeRequest{
		VolumeId: volID,
		NodeId:   nodeID,
		Secrets:  sc.Secrets.ControllerUnpublishVolumeSecret,
	}
}

// VolumeLifecycle performs Create-Publish-Unpublish-Delete, with optional repeat count to test idempotency.
func VolumeLifecycle(r *Resources, sc *TestContext, count int) {
	// CSI spec poses no specific requirements for the cluster/storage setups that a SP MUST support. To perform
	// meaningful checks the following test assumes that topology-aware provisioning on a single node setup is supported
	By("getting node information")
	ni, err := r.NodeGetInfo(
		context.Background(),
		&csi.NodeGetInfoRequest{})
	Expect(err).NotTo(HaveOccurred())
	Expect(ni).NotTo(BeNil())
	Expect(ni.GetNodeId()).NotTo(BeEmpty())

	var accReqs *csi.TopologyRequirement
	if ni.AccessibleTopology != nil {
		// Topology requirements are honored if provided by the driver
		accReqs = &csi.TopologyRequirement{
			Requisite: []*csi.Topology{ni.AccessibleTopology},
		}
	}

	// Create Volume First
	By("creating a single node writer volume")
	name := UniqueString("sanity-controller-publish")

	vol := r.MustCreateVolume(
		context.Background(),
		&csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
			},
			Secrets:                   sc.Secrets.CreateVolumeSecret,
			Parameters:                sc.Config.TestVolumeParameters,
			AccessibilityRequirements: accReqs,
		},
	)

	// ControllerPublishVolume
	for i := 0; i < count; i++ {
		By("calling controllerpublish on that volume")
		r.MustControllerPublishVolume(
			context.Background(),
			&csi.ControllerPublishVolumeRequest{
				VolumeId:         vol.GetVolume().GetVolumeId(),
				NodeId:           ni.GetNodeId(),
				VolumeCapability: TestVolumeCapabilityWithAccessType(sc, csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
				Readonly:         false,
				Secrets:          sc.Secrets.ControllerPublishVolumeSecret,
			},
		)
	}
}
