/*
Copyright 2021 The Kubernetes Authors.

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

package secretsstore

import (
	"context"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	pbSanitizer "github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// Defines Non blocking GRPC server interfaces
type NonBlockingGRPCServer interface {
	// Start services at the endpoint
	Start(ctx context.Context, endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer)
	// Waits for the service to stop
	Wait()
	// Stops the service gracefully
	Stop()
	// Stops the service forcefully
	ForceStop()
}

// NewNonBlockingGRPCServer returns a new non blocking grpc server
func NewNonBlockingGRPCServer() NonBlockingGRPCServer {
	return &nonBlockingGRPCServer{}
}

// NonBlocking server
type nonBlockingGRPCServer struct {
	wg     sync.WaitGroup
	server *grpc.Server
}

// Start the server
func (s *nonBlockingGRPCServer) Start(ctx context.Context, endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	s.wg.Add(1)
	go s.serve(ctx, endpoint, ids, cs, ns)
}

// Wait for the server to stop
func (s *nonBlockingGRPCServer) Wait() {
	s.wg.Wait()
}

// Stop the server
func (s *nonBlockingGRPCServer) Stop() {
	s.server.GracefulStop()
}

// ForceStop stops the server forcefully
func (s *nonBlockingGRPCServer) ForceStop() {
	s.server.Stop()
}

func (s *nonBlockingGRPCServer) serve(ctx context.Context, endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	defer s.wg.Done()

	proto, addr, err := parseEndpoint(endpoint)
	if err != nil {
		klog.Fatal(err.Error())
	}

	if proto == "unix" {
		if runtime.GOOS != "windows" {
			addr = "/" + addr
		}
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			klog.ErrorS(err, "failed to remove unix domain socket", "address", addr)
			os.Exit(1)
		}
	}

	listener, err := net.Listen(proto, addr)
	if err != nil {
		klog.ErrorS(err, "failed to listen", "proto", proto, "address", addr)
		os.Exit(1)
	}
	defer listener.Close()

	server := grpc.NewServer(
		grpc.UnaryInterceptor(logInterceptor()),
	)
	s.server = server

	if ids != nil {
		csi.RegisterIdentityServer(server, ids)
	}
	if cs != nil {
		csi.RegisterControllerServer(server, cs)
	}
	if ns != nil {
		csi.RegisterNodeServer(server, ns)
	}

	klog.InfoS("Listening for connections", "address", listener.Addr().String())

	go func() {
		err = server.Serve(listener)
		if err != nil {
			klog.ErrorS(err, "failed to serve")
		}
	}()

	<-ctx.Done()
	server.GracefulStop()
}

func parseEndpoint(ep string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(ep), "unix://") || strings.HasPrefix(strings.ToLower(ep), "tcp://") {
		s := strings.SplitN(ep, "://", 2)
		if s[1] != "" {
			return s[0], s[1], nil
		}
	}
	return "", "", errors.Errorf("invalid endpoint: %v", ep)
}

// logInterceptor returns a new unary server interceptors that performs request
// and response logging.
func logInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		deadline, _ := ctx.Deadline()
		dd := time.Until(deadline).String()
		klog.V(5).InfoS("request", "method", info.FullMethod, "req", pbSanitizer.StripSecrets(req).String(), "deadline", dd)

		resp, err := handler(ctx, req)
		s, _ := status.FromError(err)
		klog.V(5).InfoS("response", "method", info.FullMethod, "deadline", dd, "duration", time.Since(start).String(), "status.code", s.Code(), "status.message", s.Message())
		return resp, err
	}
}
