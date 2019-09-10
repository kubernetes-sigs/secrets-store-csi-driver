package vault

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/net/context"
	yaml "gopkg.in/yaml.v2"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"golang.org/x/net/http2"
)

const (
	// VaultObjectTypeSecret secret vault object type for HashiCorp vault
	VaultObjectTypeSecret               string = "secret"
	defaultVaultAddress                 string = "https://127.0.0.1:8200"
	defaultKubernetesServiceAccountPath string = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultVaultKubernetesMountPath     string = "kubernetes"
)

// Provider implements the secrets-store-csi-driver provider interface
// and communicates with the Vault API.
type Provider struct {
	VaultAddress                 string
	VaultCAPem                   string
	VaultCACert                  string
	VaultCAPath                  string
	VaultRole                    string
	VaultSkipVerify              bool
	VaultServerName              string
	VaultK8SMountPath            string
	KubernetesServiceAccountPath string
}

// KeyValueObject is the object stored in Vault's Key-Value store.
type KeyValueObject struct {
	// the path of the Key-Value Vault objects
	ObjectPath string `json:"objectPath" yaml:"objectPath"`
	// the name of the Key-Value Vault objects
	ObjectName string `json:"objectName" yaml:"objectName"`
	// the version of the Key-Value Vault objects
	ObjectVersion string `json:"objectVersion" yaml:"objectVersion"`
}

type StringArray struct {
	Array []string `json:"array" yaml:"array"`
}

// NewProvider creates a new provider HashiCorp Vault.
func NewProvider() (*Provider, error) {
	glog.V(2).Infof("NewVaultProvider")
	var p Provider
	return &p, nil
}

func readJWTToken(path string) (string, error) {
	glog.V(2).Infof("vault: reading jwt token.....")

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", errors.Wrap(err, "failed to read jwt token")
	}

	return string(bytes.TrimSpace(data)), nil
}

func (p *Provider) createHTTPClient() (*http.Client, error) {
	rootCAs, err := p.getRootCAsPools()
	if err != nil {
		return nil, err
	}

	tlsClientConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
	}

	if p.VaultSkipVerify {
		tlsClientConfig.InsecureSkipVerify = true
	}

	if p.VaultServerName != "" {
		tlsClientConfig.ServerName = p.VaultServerName
	}

	transport := &http.Transport{
		TLSClientConfig: tlsClientConfig,
	}

	if err := http2.ConfigureTransport(transport); err != nil {
		return nil, errors.New("failed to configure http2")
	}

	return &http.Client{
		Transport: transport,
	}, nil
}

func (p *Provider) login(jwt string, roleName string) (string, error) {
	glog.V(2).Infof("vault: performing vault login.....")

	client, err := p.createHTTPClient()
	if err != nil {
		return "", err
	}

	addr := p.VaultAddress + "/v1/auth/" + p.VaultK8SMountPath + "/login"
	body := fmt.Sprintf(`{"role": "%s", "jwt": "%s"}`, roleName, jwt)

	glog.V(2).Infof("vault: vault address: %s\n", addr)

	req, err := http.NewRequest(http.MethodPost, addr, strings.NewReader(body))
	if err != nil {
		return "", errors.Wrapf(err, "couldn't generate request")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "couldn't login")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var b bytes.Buffer
		io.Copy(&b, resp.Body)
		return "", fmt.Errorf("failed to get successful response: %#v, %s",
			resp, b.String())
	}

	var s struct {
		Auth struct {
			ClientToken string `json:"client_token"`
		} `json:"auth"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return "", errors.Wrapf(err, "failed to read body")
	}

	return s.Auth.ClientToken, nil
}

func (p *Provider) getSecret(token string, secretPath string, secretName string, secretVersion string) (string, error) {
	glog.V(2).Infof("vault: getting secrets from vault.....")

	client, err := p.createHTTPClient()
	if err != nil {
		return "", err
	}

	if secretVersion == "" {
		secretVersion = "0"
	}

	addr := p.VaultAddress + "/v1/secret/data" + secretPath + "?version=" + secretVersion

	req, err := http.NewRequest(http.MethodGet, addr, nil)
	// Set vault token.
	req.Header.Set("X-Vault-Token", token)
	if err != nil {
		return "", errors.Wrapf(err, "couldn't generate request")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "couldn't get secret")
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var b bytes.Buffer
		io.Copy(&b, resp.Body)
		return "", fmt.Errorf("failed to get successful response: %#v, %s",
			resp, b.String())
	}

	var d struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return "", errors.Wrapf(err, "failed to read body")
	}

	return d.Data.Data[secretName], nil
}

func (p *Provider) getRootCAsPools() (*x509.CertPool, error) {
	switch {
	case p.VaultCAPem != "":
		certPool := x509.NewCertPool()
		if err := loadCert(certPool, []byte(p.VaultCAPem)); err != nil {
			return nil, err
		}
		return certPool, nil
	case p.VaultCAPath != "":
		certPool := x509.NewCertPool()
		if err := loadCertFolder(certPool, p.VaultCAPath); err != nil {
			return nil, err
		}
		return certPool, nil
	case p.VaultCACert != "":
		certPool := x509.NewCertPool()
		if err := loadCertFile(certPool, p.VaultCACert); err != nil {
			return nil, err
		}
		return certPool, nil
	default:
		certPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't load system certs")
		}
		return certPool, err
	}
}

// loadCert loads a single pem-encoded certificate into the given pool.
func loadCert(pool *x509.CertPool, pem []byte) error {
	if ok := pool.AppendCertsFromPEM(pem); !ok {
		return fmt.Errorf("failed to parse PEM")
	}
	return nil
}

// loadCertFile loads the certificate at the given path into the given pool.
func loadCertFile(pool *x509.CertPool, p string) error {
	pem, err := ioutil.ReadFile(p)
	if err != nil {
		return errors.Wrapf(err, "couldn't read CA file from disk")
	}

	if err := loadCert(pool, pem); err != nil {
		return errors.Wrapf(err, "couldn't load CA at %s", p)
	}

	return nil
}

// loadCertFolder iterates exactly one level below the given directory path and
// loads all certificates in that path. It does not recurse
func loadCertFolder(pool *x509.CertPool, p string) error {
	if err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		return loadCertFile(pool, path)
	}); err != nil {
		return errors.Wrapf(err, "failed to load CAs at %s", p)
	}
	return nil
}

// MountSecretsStoreObjectContent mounts content of the vault object to target path
func (p *Provider) MountSecretsStoreObjectContent(ctx context.Context, attrib map[string]string, secrets map[string]string, targetPath string, permission os.FileMode) (err error) {
	roleName := attrib["roleName"]
	if roleName == "" {
		return errors.Errorf("missing vault role name. please specify 'roleName' in pv definition.")
	}
	p.VaultRole = roleName

	glog.V(2).Infof("vault: roleName %s", p.VaultRole)

	p.VaultAddress = attrib["vaultAddress"]
	if p.VaultAddress == "" {
		p.VaultAddress = defaultVaultAddress
	}
	glog.V(2).Infof("vault: vault address %s", p.VaultAddress)

	// One of the following variables should be set when vaultSkipTLSVerify is false.
	// Otherwise, system certificates are used to make requests to vault.
	p.VaultCAPem = attrib["vaultCAPem"]
	p.VaultCACert = attrib["vaultCACertPath"]
	p.VaultCAPath = attrib["vaultCADirectory"]
	// Vault tls server name.
	p.VaultServerName = attrib["vaultTLSServerName"]

	if s := attrib["vaultSkipTLSVerify"]; s != "" {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		p.VaultSkipVerify = b
	}

	p.VaultK8SMountPath = attrib["vaultKubernetesMountPath"]
	if p.VaultK8SMountPath == "" {
		p.VaultK8SMountPath = defaultVaultKubernetesMountPath
	}

	p.KubernetesServiceAccountPath = attrib["vaultKubernetesServiceAccountPath"]
	if p.KubernetesServiceAccountPath == "" {
		p.KubernetesServiceAccountPath = defaultKubernetesServiceAccountPath
	}

	keyValueObjects := []KeyValueObject{}
	objectsStrings := attrib["objects"]
	fmt.Printf("objectsStrings: [%s]\n", objectsStrings)

	var objects StringArray
	err = yaml.Unmarshal([]byte(objectsStrings), &objects)
	if err != nil {
		fmt.Printf("unmarshall failed for objects")
		return err
	}
	fmt.Printf("objects: [%v]", objects.Array)
	for _, object := range objects.Array {
		fmt.Printf("unmarshal object: [%s]\n", object)
		var keyValueObject KeyValueObject
		err = yaml.Unmarshal([]byte(object), &keyValueObject)
		if err != nil {
			fmt.Printf("unmarshall failed for keyValueObjects at index")
			return err
		}

		keyValueObjects = append(keyValueObjects, keyValueObject)
	}

	for _, keyValueObject := range keyValueObjects {
		content, err := p.GetKeyValueObjectContent(ctx, keyValueObject.ObjectPath, keyValueObject.ObjectName, keyValueObject.ObjectVersion)
		if err != nil {
			return err
		}
		objectContent := []byte(content)
		if err := ioutil.WriteFile(path.Join(targetPath, keyValueObject.ObjectPath), objectContent, permission); err != nil {
			return errors.Wrapf(err, "secrets-store csi driver failed to write %s at %s", keyValueObject.ObjectPath, targetPath)
		}
		glog.V(0).Infof("secrets-store csi driver wrote %s at %s", keyValueObject.ObjectPath, targetPath)
	}

	return nil
}

// GetKeyValueObjectContent get content of the vault object
func (p *Provider) GetKeyValueObjectContent(ctx context.Context, objectPath string, objectName string, objectVersion string) (content string, err error) {
	// Read the jwt token from disk
	jwt, err := readJWTToken(p.KubernetesServiceAccountPath)
	if err != nil {
		return "", err
	}

	// Authenticate to vault using the jwt token
	token, err := p.login(jwt, p.VaultRole)
	if err != nil {
		return "", err
	}

	// Get Secret
	value, err := p.getSecret(token, objectPath, objectName, objectVersion)
	if err != nil {
		return "", err
	}

	return value, nil
}
