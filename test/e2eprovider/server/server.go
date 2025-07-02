//go:build e2e
// +build e2e

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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2eprovider/types"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

var (
	secrets = map[string]string{
		"foo": "secret",
		"fookey": `-----BEGIN PUBLIC KEY-----
This is mock key
-----END PUBLIC KEY-----`,
	}

	// podCache is a map of pod UID to check if secret has been rotated.
	podCache = map[string]bool{}

	podUIDAttribute               = "csi.storage.k8s.io/pod.uid"
	serviceAccountTokensAttribute = "csi.storage.k8s.io/serviceAccount.tokens" //nolint

	// RWMutex is to safely access podCache
	m sync.RWMutex
)

// Server is a mock csi-provider server
type Server struct {
	grpcServer *grpc.Server
	socketPath string
	network    string
}

// NewE2EProviderServer returns a mock csi-provider grpc server
func NewE2EProviderServer(endpoint string) (*Server, error) {
	var network, address string

	if strings.HasPrefix(strings.ToLower(endpoint), "unix://") || strings.HasPrefix(strings.ToLower(endpoint), "tcp://") {
		s := strings.SplitN(endpoint, "://", 2)
		if s[1] != "" {
			network = s[0]
			address = s[1]
		} else {
			return nil, fmt.Errorf("invalid endpoint: %s", endpoint)
		}
	}

	server := grpc.NewServer()
	s := &Server{
		grpcServer: server,
		socketPath: address,
		network:    network,
	}

	v1alpha1.RegisterCSIDriverProviderServer(server, s)

	return s, nil
}

// GetSocketPath returns the socket path
func (s *Server) GetSocketPath() string {
	return s.socketPath
}

// Start starts the mock csi-provider server
func (s *Server) Start() error {
	listener, err := net.Listen(s.network, s.GetSocketPath())
	if err != nil {
		return err
	}

	klog.InfoS("Listening for connections", "address", listener.Addr())
	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			return
		}
	}()
	return nil
}

// Stop stops the mock csi-provider server
func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
}

// Mount implements provider csi-provider method
func (s *Server) Mount(ctx context.Context, req *v1alpha1.MountRequest) (*v1alpha1.MountResponse, error) {
	var attrib, secret map[string]string
	var filePermission os.FileMode
	var err error

	resp := &v1alpha1.MountResponse{
		ObjectVersion: []*v1alpha1.ObjectVersion{},
	}

	if err = json.Unmarshal([]byte(req.GetAttributes()), &attrib); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attributes, error: %w", err)
	}
	if err = json.Unmarshal([]byte(req.GetSecrets()), &secret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets, error: %w", err)
	}
	if err = json.Unmarshal([]byte(req.GetPermission()), &filePermission); err != nil {
		return nil, fmt.Errorf("failed to unmarshal file permission, error: %w", err)
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, fmt.Errorf("missing target path")
	}

	objectsStrings := attrib["objects"]
	if objectsStrings == "" {
		return nil, fmt.Errorf("objects is not set")
	}

	var objects types.StringArray
	err = yaml.Unmarshal([]byte(objectsStrings), &objects)
	if err != nil {
		return nil, fmt.Errorf("failed to yaml unmarshal objects, error: %w", err)
	}

	mockSecretsStoreObjects := []types.MockSecretsStoreObject{}
	for i, object := range objects.Array {
		var mockSecretsStoreObject types.MockSecretsStoreObject
		err = yaml.Unmarshal([]byte(object), &mockSecretsStoreObject)
		if err != nil {
			return nil, fmt.Errorf("unmarshal failed for keyVaultObjects at index %d, error: %w", i, err)
		}

		mockSecretsStoreObjects = append(mockSecretsStoreObjects, mockSecretsStoreObject)
	}

	for _, mockSecretsStoreObject := range mockSecretsStoreObjects {
		secretFile, version, err := getSecret(mockSecretsStoreObject.ObjectName, attrib[podUIDAttribute])
		if err != nil {
			return nil, fmt.Errorf("failed to get secret, error: %w", err)
		}

		klog.InfoS("Secret Object with", "name", mockSecretsStoreObject.ObjectName, "permission", mockSecretsStoreObject.FilePermission)
		if mockSecretsStoreObject.FilePermission != "" {
			mode, err := strconv.ParseUint(mockSecretsStoreObject.FilePermission, 8, 32)
			if err != nil || mode > 511 {
				return nil, fmt.Errorf("invalid filePermission: %s, error: %w for file: %s", mockSecretsStoreObject.FilePermission, err, mockSecretsStoreObject.ObjectName)
			}
			secretFile.Mode = int32(mode)
			klog.InfoS("Set file mode", "file", secretFile.Path, "mode", os.FileMode(mode))
		}
		resp.Files = append(resp.Files, secretFile)
		resp.ObjectVersion = append(resp.ObjectVersion, version)
	}

	// if validate token flag is set, we want to check the service account tokens as passed
	// as part of the mount attributes.
	// In case of 1.21+, kubelet will generate the token and pass it as part of the volume context.
	// The driver will pass this to the provider as part of the mount request.
	// For 1.20, the driver will generate the token and pass it to the provider as part of the mount request.
	// Irrespective of the kubernetes version, the rotation handler in the driver will generate the token
	// and pass it to the provider as part of the mount request.
	// VALIDATE_TOKENS_AUDIENCE environment variable will be a comma separated list of audiences configured in the csidriver object
	// If this env var is not set, this could mean we are running an older version of driver.
	tokenAudiences := os.Getenv("VALIDATE_TOKENS_AUDIENCE")
	klog.InfoS("tokenAudiences", "tokenAudiences", tokenAudiences)
	if tokenAudiences != "" {
		if err := validateTokens(tokenAudiences, attrib[serviceAccountTokensAttribute]); err != nil {
			return nil, fmt.Errorf("failed to validate token, error: %w", err)
		}
	}

	m.Lock()
	podCache[attrib[podUIDAttribute]] = true
	m.Unlock()

	return resp, nil
}

func getSecret(secretName, podUID string) (*v1alpha1.File, *v1alpha1.ObjectVersion, error) {
	secretVersion := "v1"
	secretContent := secrets[secretName]

	// If pod found in cache, then it means that pod is being called for the second time for rotation
	// In this case, we should return the 'rotated' secret.
	m.RLock()
	if ok := podCache[podUID]; ok {
		if os.Getenv("ROTATION_ENABLED") == "true" {
			// ROTAION_ENABLED is set to true only when rotation tests are running
			secretVersion = "v2"
			secretContent = "rotated"
		}
	}
	m.RUnlock()

	secretFile := &v1alpha1.File{
		Path:     secretName,
		Contents: []byte(secretContent),
	}

	version := &v1alpha1.ObjectVersion{
		Id:      fmt.Sprintf("secret/%s", secretName),
		Version: secretVersion,
	}

	return secretFile, version, nil
}

// Version implements provider csi-provider method
func (s *Server) Version(ctx context.Context, req *v1alpha1.VersionRequest) (*v1alpha1.VersionResponse, error) {
	return &v1alpha1.VersionResponse{
		Version:        "v1alpha1",
		RuntimeName:    "E2EMockProvider",
		RuntimeVersion: "v0.0.10",
	}, nil
}

// RotationHandler enables rotation response for the mock provider
func RotationHandler(w http.ResponseWriter, r *http.Request) {
	// enable rotation response
	os.Setenv("ROTATION_ENABLED", r.FormValue("rotated"))
	klog.InfoS("Rotation response enabled")
}

// ValidateTokenAudienceHandler enables token validation for the mock provider
// This is only required because older version of the driver don't generate a token
// TODO(aramase): remove this after the supported driver releases are v1.1.0+
func ValidateTokenAudienceHandler(w http.ResponseWriter, r *http.Request) {
	// enable rotation response
	os.Setenv("VALIDATE_TOKENS_AUDIENCE", r.FormValue("audience"))
	klog.InfoS("Validation for token requests audience", "audience", os.Getenv("VALIDATE_TOKENS_AUDIENCE"))
}

// validateTokens checks there are tokens for distinct audiences in the
// service account token attribute.
func validateTokens(tokenAudiences, saTokens string) error {
	ta := strings.Split(strings.TrimSpace(tokenAudiences), ",")
	if saTokens == "" {
		return fmt.Errorf("service account tokens is not set")
	}
	tokens := make(map[string]interface{})
	if err := json.Unmarshal([]byte(saTokens), &tokens); err != nil {
		return fmt.Errorf("failed to unmarshal service account tokens, error: %w", err)
	}
	for _, a := range ta {
		if _, ok := tokens[a]; !ok {
			return fmt.Errorf("service account token for audience %s is not set", a)
		}
		klog.InfoS("Validated service account token", "audience", a)
	}
	return nil
}
