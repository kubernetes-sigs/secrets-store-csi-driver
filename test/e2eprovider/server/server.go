package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"
	util "sigs.k8s.io/secrets-store-csi-driver/pkg/csi-common"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
	"sigs.k8s.io/yaml"
)

// SimpleSecretKeyValue struct represents a secret key value pair
type SimpleSecretKeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// KubernetesTokenContent struct represents a kubernetes token
type KubernetesTokenContent struct {
	Token               string    `json:"token"`
	ExpirationTimestamp time.Time `json:"expirationTimestamp"`
}

// SimpleCSIProviderServer is a mock csi-provider server
type SimpleCSIProviderServer struct {
	grpcServer *grpc.Server
	listener   net.Listener
	SocketPath string
	network    string
}

// NewSimpleCSIProviderServer returns a mock csi-provider grpc server
func NewSimpleCSIProviderServer(endpoint string) (*SimpleCSIProviderServer, error) {
	network, address, err := util.ParseEndpoint(endpoint)
	if err != nil {
		klog.Fatal(err.Error())
	}

	server := grpc.NewServer()
	s := &SimpleCSIProviderServer{
		grpcServer: server,
		SocketPath: address,
		network:    network,
	}
	v1alpha1.RegisterCSIDriverProviderServer(server, s)
	return s, nil
}

// Start starts the mock csi-provider server
func (m *SimpleCSIProviderServer) Start() error {
	var err error
	m.listener, err = net.Listen(m.network, m.SocketPath)
	if err != nil {
		return err
	}
	go m.grpcServer.Serve(m.listener)
	return nil
}

// Stop stops the mock csi-provider server
func (m *SimpleCSIProviderServer) Stop() {
	m.grpcServer.GracefulStop()
}

// Mount implements provider csi-provider method
func (m *SimpleCSIProviderServer) Mount(ctx context.Context, req *v1alpha1.MountRequest) (*v1alpha1.MountResponse, error) {
	var attrib, secret map[string]string
	var filePermission os.FileMode
	var err error

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

	resp := &v1alpha1.MountResponse{
		ObjectVersion: []*v1alpha1.ObjectVersion{},
	}

	if rawTokenContent, ok := attrib["csi.storage.k8s.io/serviceAccount.tokens"]; ok {
		tokens := map[string]KubernetesTokenContent{}
		err := json.Unmarshal([]byte(rawTokenContent), &tokens)
		if err != nil {
			klog.Errorf("Error unmarshaling tokens attribute: %v", err)
		}
		files := []*v1alpha1.File{}
		for sub, content := range tokens {
			u, _ := url.Parse(sub)

			path := filepath.Join(u.Hostname(), u.EscapedPath())
			files = append(files, &v1alpha1.File{
				Path:     path,
				Contents: []byte(content.Token),
			})
			resp.ObjectVersion = append(resp.ObjectVersion, &v1alpha1.ObjectVersion{Id: fmt.Sprintf("secret/%s", path), Version: "v1"})
		}
		resp.Files = append(resp.Files, files...)

	}
	if rawSecretContent, ok := attrib["secrets"]; ok {
		secretContents := []SimpleSecretKeyValue{}
		err := yaml.Unmarshal([]byte(rawSecretContent), &secretContents)
		if err != nil {
			klog.Errorf("Error unmarshaling secret attribute: %v", err)
		}

		files := []*v1alpha1.File{}
		for _, kv := range secretContents {
			files = append(files, &v1alpha1.File{
				Path:     kv.Key,
				Contents: []byte(kv.Value),
			})
			resp.ObjectVersion = append(resp.ObjectVersion, &v1alpha1.ObjectVersion{Id: fmt.Sprintf("secret/%s", kv.Key), Version: "v1"})
		}
		resp.Files = append(resp.Files, files...)
	}
	return resp, nil
}

// Version implements provider csi-provider method
func (m *SimpleCSIProviderServer) Version(ctx context.Context, req *v1alpha1.VersionRequest) (*v1alpha1.VersionResponse, error) {
	return &v1alpha1.VersionResponse{
		Version:        "v1alpha1",
		RuntimeName:    "SimpleProvider",
		RuntimeVersion: "0.0.10",
	}, nil
}
