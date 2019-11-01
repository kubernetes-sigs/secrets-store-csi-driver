/*
Copyright 2017 The Kubernetes Authors.

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

package csicommon

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func ParseEndpoint(ep string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(ep), "unix://") || strings.HasPrefix(strings.ToLower(ep), "tcp://") {
		s := strings.SplitN(ep, "://", 2)
		if s[1] != "" {
			return s[0], s[1], nil
		}
	}
	return "", "", fmt.Errorf("Invalid endpoint: %v", ep)
}

func NewVolumeCapabilityAccessMode(mode csi.VolumeCapability_AccessMode_Mode) *csi.VolumeCapability_AccessMode {
	return &csi.VolumeCapability_AccessMode{Mode: mode}
}

func NewDefaultNodeServer(d *CSIDriver) *DefaultNodeServer {
	return &DefaultNodeServer{
		Driver: d,
	}
}

func NewDefaultIdentityServer(d *CSIDriver) *DefaultIdentityServer {
	return &DefaultIdentityServer{
		Driver: d,
	}
}

func NewDefaultControllerServer(d *CSIDriver) *DefaultControllerServer {
	return &DefaultControllerServer{
		Driver: d,
	}
}

func NewControllerServiceCapability(cap csi.ControllerServiceCapability_RPC_Type) *csi.ControllerServiceCapability {
	return &csi.ControllerServiceCapability{
		Type: &csi.ControllerServiceCapability_Rpc{
			Rpc: &csi.ControllerServiceCapability_RPC{
				Type: cap,
			},
		},
	}
}

func RunNodePublishServer(endpoint string, d *CSIDriver, ns csi.NodeServer) {
	ids := NewDefaultIdentityServer(d)

	s := NewNonBlockingGRPCServer()
	s.Start(endpoint, ids, nil, ns)
	s.Wait()
}

func RunControllerPublishServer(endpoint string, d *CSIDriver, cs csi.ControllerServer) {
	ids := NewDefaultIdentityServer(d)

	s := NewNonBlockingGRPCServer()
	s.Start(endpoint, ids, cs, nil)
	s.Wait()
}

func RunControllerandNodePublishServer(endpoint string, d *CSIDriver, cs csi.ControllerServer, ns csi.NodeServer) {
	ids := NewDefaultIdentityServer(d)

	s := NewNonBlockingGRPCServer()
	s.Start(endpoint, ids, cs, ns)
	s.Wait()
}

func logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	log.Debugf("GRPC call: %s", info.FullMethod)
	logRedactedRequest(req)
	resp, err := handler(ctx, req)
	if err != nil {
		log.Errorf("GRPC error: %v", err)
	} else {
		log.Debugf("GRPC response: %+v", resp)
	}
	return resp, err
}

func logRedactedRequest(req interface{}) {
	r, ok := req.(*csi.NodePublishVolumeRequest)
	if !ok {
		log.Debugf("GRPC request: %+v", req)
		return
	}

	req1 := *r
	redactedSecrets := make(map[string]string)

	secrets := req1.GetSecrets()
	for k := range secrets {
		redactedSecrets[k] = "[REDACTED]"
	}
	req1.Secrets = redactedSecrets
	log.Debugf("GRPC request: %+v", req1)
}
