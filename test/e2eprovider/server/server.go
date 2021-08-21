// +build e2e

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

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
)

func getSecret(objectName, podUID string) (*v1alpha1.SecretFile, *v1alpha1.ObjectVersion, error) {
	var secretFile v1alpha1.SecretFile
	var version v1alpha1.ObjectVersion

	secretFile.Name = objectName
	secretFile.Data = secrets[objectName]
	secretFile.Permission = os.FileMode(0644)

	version.Name = objectName
	version.Data = podUID

	return &secretFile, &version, nil
}
	podCache = make(map[string]bool)

	podUIDAttribute = "csi.storage.k8s.io/pod.uid"
)

// Server is a mock csi-provider server
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	SocketPath string
	network    string
}

// NewE2EProviderServer returns a mock csi-provider grpc server
func NewE2EProviderServer(endpoint string) (*Server, error) {
	network := ""
	address := ""
	if strings.HasPrefix(strings.ToLower(endpoint), "unix://") || strings.HasPrefix(strings.ToLower(endpoint), "tcp://") {
		s := strings.SplitN(endpoint, "://", 2)
		if s[1] != "" {
			network = s[0]
			address = s[1]
		} else {
			return nil, fmt.Errorf("Invalid endpoint: %s", endpoint)
		}
	}

	server := grpc.NewServer()
	s := &Server{
		grpcServer: server,
		SocketPath: address,
		network:    network,
	}

	v1alpha1.RegisterCSIDriverProviderServer(server, s)

	return s, nil
}

// Start starts the mock csi-provider server
func (m *Server) Start() error {
	var err error

	m.listener, err = net.Listen(m.network, m.SocketPath)
	if err != nil {
		return err
	}

	klog.InfoS("Listening for connections on address:", m.listener.Addr())
	go m.grpcServer.Serve(m.listener)
	return nil
}

// Stop stops the mock csi-provider server
func (m *Server) Stop() {
	m.grpcServer.GracefulStop()
}

// Mount implements provider csi-provider method
func (m *Server) Mount(ctx context.Context, req *v1alpha1.MountRequest) (*v1alpha1.MountResponse, error) {
	var attrib, secret map[string]string
	var filePermission os.FileMode
	var err error

	resp := &v1alpha1.MountResponse{
		ObjectVersion: []*v1alpha1.ObjectVersion{},
	}

	if err = json.Unmarshal([]byte(req.GetAttributes()), &attrib); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attributes, error: %+v", err)
	}
	if err = json.Unmarshal([]byte(req.GetSecrets()), &secret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets, error: %+v", err)
	}
	if err = json.Unmarshal([]byte(req.GetPermission()), &filePermission); err != nil {
		return nil, fmt.Errorf("failed to unmarshal file permission, error: %+v", err)
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

		resp.Files = append(resp.Files, secretFile)
		resp.ObjectVersion = append(resp.ObjectVersion, version)
	}
	podCache[attrib[podUIDAttribute]] = true

	return resp, nil
}

func getSecret(secretName, podUID string) (*v1alpha1.File, *v1alpha1.ObjectVersion, error) {
	secretVersion := "v1"
	secretContent := secrets[secretName]

	// If pod found in cache, then it means that pod is being called for the second time for rotation
	// In this case, we should return the 'rotated' secret.
	if ok := podCache[podUID]; ok {
		secretVersion = "v2"
		secretContent = "rotated"
	}

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
func (m *Server) Version(ctx context.Context, req *v1alpha1.VersionRequest) (*v1alpha1.VersionResponse, error) {
	return &v1alpha1.VersionResponse{
		Version:        "v1alpha1",
		RuntimeName:    "SimpleProvider",
		RuntimeVersion: "0.0.10",
	}, nil
}
