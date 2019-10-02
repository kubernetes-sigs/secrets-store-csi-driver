/*
Copyright 2018 The Kubernetes Authors.

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

package azure

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"golang.org/x/net/context"
	yaml "gopkg.in/yaml.v2"

	kv "github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	kvmgmt "github.com/Azure/azure-sdk-for-go/services/keyvault/mgmt/2018-02-14/keyvault"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

// Type of Azure Key Vault objects
const (
	// VaultObjectTypeSecret secret vault object type
	VaultObjectTypeSecret string = "secret"
	// VaultObjectTypeKey key vault object type
	VaultObjectTypeKey string = "key"
	// VaultObjectTypeCertificate certificate vault object type
	VaultObjectTypeCertificate string = "cert"
	// OAuthGrantTypeServicePrincipal for client credentials flow
	OAuthGrantTypeServicePrincipal OAuthGrantType = iota
	// OAuthGrantTypeDeviceFlow for device-auth flow
	OAuthGrantTypeDeviceFlow
	// Pod Identity nmiendpoint
	nmiendpoint = "http://localhost:2579/host/token/"
	// Pod Identity podnameheader
	podnameheader = "podname"
	// Pod Identity podnsheader
	podnsheader = "podns"
)

// NMIResponse is the response received from aad-pod-identity
type NMIResponse struct {
	Token    adal.Token `json:"token"`
	ClientID string     `json:"clientid"`
}

// OAuthGrantType specifies which grant type to use.
type OAuthGrantType int

// AuthGrantType ...
func AuthGrantType() OAuthGrantType {
	return OAuthGrantTypeServicePrincipal
}

// Provider implements the secrets-store-csi-driver provider interface
type Provider struct {
	// the name of the Azure Key Vault instance
	KeyvaultName string
	// the name of the Azure Key Vault objects, since attributes can only be strings, this will be mapped to StringArray, which is an array of KeyVaultObject
	Objects []KeyVaultObject
	// the resourcegroup of the Azure Key Vault
	ResourceGroup string
	// subscriptionId to azure
	SubscriptionID string
	// tenantID in AAD
	TenantID string
	// POD AAD Identity flag
	UsePodIdentity bool
	// AAD app client secret (if not using POD AAD Identity)
	AADClientSecret string
	// AAD app client secret id (if not using POD AAD Identity)
	AADClientID string
	// the name of the pod (if using POD AAD Identity)
	PodName string
	// the namespace of the pod (if using POD AAD Identity)
	PodNamespace string
	// the name of the azure cloud
	CloudName string
}

// KeyVaultObject holds keyvault object related config
type KeyVaultObject struct {
	// the name of the Azure Key Vault objects
	ObjectName string `json:"objectName" yaml:"objectName"`
	// the version of the Azure Key Vault objects
	ObjectVersion string `json:"objectVersion" yaml:"objectVersion"`
	// the type of the Azure Key Vault objects
	ObjectType string `json:"objectType" yaml:"objectType"`
}

// StringArray ...
type StringArray struct {
	Array []string `json:"array" yaml:"array"`
}

// NewProvider creates a new Azure Key Vault Provider.
func NewProvider() (*Provider, error) {
	glog.V(2).Infof("NewAzureProvider")
	var p Provider
	return &p, nil
}

// ParseAzureEnvironment returns azure environment by name
func ParseAzureEnvironment(cloudName string) (*azure.Environment, error) {
	var env azure.Environment
	var err error
	if cloudName == "" {
		env = azure.PublicCloud
	} else {
		env, err = azure.EnvironmentFromName(cloudName)
	}
	return &env, err
}

// GetKeyvaultToken retrieves a new service principal token to access keyvault
func (p *Provider) GetKeyvaultToken(grantType OAuthGrantType) (authorizer autorest.Authorizer, err error) {
	env, err := ParseAzureEnvironment(p.CloudName)
	if err != nil {
		return nil, err
	}

	kvEndPoint := env.KeyVaultEndpoint
	if '/' == kvEndPoint[len(kvEndPoint)-1] {
		kvEndPoint = kvEndPoint[:len(kvEndPoint)-1]
	}
	servicePrincipalToken, err := p.GetServicePrincipalToken(env, kvEndPoint)
	if err != nil {
		return nil, err
	}
	authorizer = autorest.NewBearerAuthorizer(servicePrincipalToken)
	return authorizer, nil
}

func (p *Provider) initializeKvClient() (*kv.BaseClient, error) {
	kvClient := kv.New()
	token, err := p.GetKeyvaultToken(AuthGrantType())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get key vault token")
	}

	kvClient.Authorizer = token
	return &kvClient, nil
}

// GetCredential gets clientid and clientsecret
func GetCredential(secrets map[string]string) (string, string, error) {
	if secrets == nil {
		return "", "", fmt.Errorf("unexpected: getCredential failed, nodePublishSecretRef secret is not provided")
	}

	var clientID, clientSecret string
	for k, v := range secrets {
		switch strings.ToLower(k) {
		case "clientid":
			clientID = v
		case "clientsecret":
			clientSecret = v
		}
	}

	if clientID == "" {
		return "", "", fmt.Errorf("could not find clientid in secrets(%v)", secrets)
	}
	if clientSecret == "" {
		return "", "", fmt.Errorf("could not find clientsecret in secrets(%v)", secrets)
	}

	return clientID, clientSecret, nil
}

func (p *Provider) getVaultURL(ctx context.Context) (vaultURL *string, err error) {
	glog.V(5).Infof("subscriptionID: %s", p.SubscriptionID)
	glog.V(5).Infof("vaultName: %s", p.KeyvaultName)
	glog.V(5).Infof("resourceGroup: %s", p.ResourceGroup)

	vaultsClient := kvmgmt.NewVaultsClient(p.SubscriptionID)
	token, tokenErr := p.GetManagementToken(AuthGrantType())
	if tokenErr != nil {
		return nil, errors.Wrapf(tokenErr, "failed to get management token")
	}
	vaultsClient.Authorizer = token
	vault, err := vaultsClient.Get(ctx, p.ResourceGroup, p.KeyvaultName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get vault %s", p.KeyvaultName)
	}
	return vault.Properties.VaultURI, nil
}

// GetManagementToken retrieves a new service principal token
func (p *Provider) GetManagementToken(grantType OAuthGrantType) (authorizer autorest.Authorizer, err error) {

	env, err := ParseAzureEnvironment(p.CloudName)
	if err != nil {
		return nil, err
	}

	rmEndPoint := env.ResourceManagerEndpoint
	servicePrincipalToken, err := p.GetServicePrincipalToken(env, rmEndPoint)
	if err != nil {
		return nil, err
	}
	authorizer = autorest.NewBearerAuthorizer(servicePrincipalToken)
	return authorizer, nil
}

// GetServicePrincipalToken creates a new service principal token based on the configuration
func (p *Provider) GetServicePrincipalToken(env *azure.Environment, resource string) (*adal.ServicePrincipalToken, error) {
	oauthConfig, err := adal.NewOAuthConfig(env.ActiveDirectoryEndpoint, p.TenantID)
	if err != nil {
		return nil, fmt.Errorf("creating the OAuth config: %v", err)
	}

	// For usepodidentity mode, the CSI driver makes an authorization request to fetch token for a resource from the NMI host endpoint (http://127.0.0.1:2579/host/token/).
	// The request includes the pod namespace `podns` and the pod name `podname` in the request header and the resource endpoint of the resource requesting the token.
	// The NMI server identifies the pod based on the `podns` and `podname` in the request header and then queries k8s (through MIC) for a matching azure identity.
	// Then nmi makes an adal request to get a token for the resource in the request, returns the `token` and the `clientid` as a response to the CSI request.

	if p.UsePodIdentity {
		glog.V(0).Infof("azure: using pod identity to retrieve token")

		endpoint := fmt.Sprintf("%s?resource=%s", nmiendpoint, resource)
		client := &http.Client{}
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Add(podnsheader, p.PodNamespace)
		req.Header.Add(podnameheader, p.PodName)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			bodyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			var nmiResp = new(NMIResponse)
			err = json.Unmarshal(bodyBytes, &nmiResp)
			if err != nil {
				return nil, err
			}

			r, _ := regexp.Compile(`^(\S{4})(\S|\s)*(\S{4})$`)
			glog.V(0).Infof("accesstoken: %s", r.ReplaceAllString(nmiResp.Token.AccessToken, "$1##### REDACTED #####$3"))
			glog.V(0).Infof("clientid: %s", r.ReplaceAllString(nmiResp.ClientID, "$1##### REDACTED #####$3"))

			token := nmiResp.Token
			clientID := nmiResp.ClientID

			if token.AccessToken == "" || clientID == "" {
				return nil, fmt.Errorf("nmi did not return expected values in response: token and clientid")
			}

			spt, err := adal.NewServicePrincipalTokenFromManualToken(*oauthConfig, clientID, resource, token, nil)
			if err != nil {
				return nil, err
			}
			return spt, nil
		}

		err = fmt.Errorf("nmi response failed with status code: %d", resp.StatusCode)
		return nil, err
	}
	// When CSI driver is using a Service Principal clientid + client secret to retrieve token for resource
	if len(p.AADClientSecret) > 0 {
		glog.V(2).Infof("azure: using client_id+client_secret to retrieve access token")
		return adal.NewServicePrincipalToken(
			*oauthConfig,
			p.AADClientID,
			p.AADClientSecret,
			resource)
	}
	return nil, fmt.Errorf("No credentials provided for AAD application %s", p.AADClientID)
}

// MountSecretsStoreObjectContent mounts content of the secrets store object to target path
func (p *Provider) MountSecretsStoreObjectContent(ctx context.Context, attrib map[string]string, secrets map[string]string, targetPath string, permission os.FileMode) (err error) {
	keyvaultName := attrib["keyvaultName"]
	usePodIdentityStr := attrib["usePodIdentity"]
	resourceGroup := attrib["resourceGroup"]
	subscriptionID := attrib["subscriptionId"]
	tenantID := attrib["tenantId"]
	p.PodName = attrib["csi.storage.k8s.io/pod.name"]
	p.PodNamespace = attrib["csi.storage.k8s.io/pod.namespace"]
	p.CloudName = attrib["cloudName"]

	if keyvaultName == "" {
		return fmt.Errorf("keyvaultName is not set")
	}
	if resourceGroup == "" {
		return fmt.Errorf("resourceGroup is not set")
	}
	if subscriptionID == "" {
		return fmt.Errorf("subscriptionId is not set")
	}
	if tenantID == "" {
		return fmt.Errorf("tenantId is not set")
	}
	// defaults
	usePodIdentity := false
	if usePodIdentityStr == "true" {
		usePodIdentity = true
	}
	if !usePodIdentity {
		glog.V(0).Infof("not using pod identity to access keyvault")
		p.AADClientID, p.AADClientSecret, err = GetCredential(secrets)
		if err != nil {
			glog.V(0).Infof("missing client credential to access keyvault")
			return err
		}
	} else {
		glog.V(0).Infof("using pod identity to access keyvault")
		if p.PodName == "" || p.PodNamespace == "" {
			return fmt.Errorf("pod information is not available. deploy a CSIDriver object to set podInfoOnMount")
		}
	}
	objectsStrings := attrib["objects"]
	if objectsStrings == "" {
		return fmt.Errorf("objects is not set")
	}
	glog.V(5).Infof("objects: %s", objectsStrings)

	var objects StringArray
	err = yaml.Unmarshal([]byte(objectsStrings), &objects)
	if err != nil {
		glog.V(0).Infof("unmarshal failed for objects")
		return err
	}
	glog.V(5).Infof("objects array: %v", objects.Array)
	keyVaultObjects := []KeyVaultObject{}
	for i, object := range objects.Array {
		var keyVaultObject KeyVaultObject
		err = yaml.Unmarshal([]byte(object), &keyVaultObject)
		if err != nil {
			glog.V(0).Infof("unmarshal failed for keyVaultObjects at index %d", i)
			return err
		}
		keyVaultObjects = append(keyVaultObjects, keyVaultObject)
	}

	glog.V(5).Infof("unmarshaled keyVaultObjects: %v", keyVaultObjects)
	glog.V(0).Infof("keyVaultObjects len: %d", len(keyVaultObjects))

	if len(keyVaultObjects) == 0 {
		return fmt.Errorf("objects array is empty")
	}
	p.KeyvaultName = keyvaultName
	p.UsePodIdentity = usePodIdentity
	p.ResourceGroup = resourceGroup
	p.SubscriptionID = subscriptionID
	p.TenantID = tenantID

	for _, keyVaultObject := range keyVaultObjects {
		content, err := p.GetKeyVaultObjectContent(ctx, keyVaultObject.ObjectType, keyVaultObject.ObjectName, keyVaultObject.ObjectVersion)
		if err != nil {
			return err
		}
		objectContent := []byte(content)
		if err := ioutil.WriteFile(path.Join(targetPath, keyVaultObject.ObjectName), objectContent, permission); err != nil {
			return errors.Wrapf(err, "Secrets Store csi driver failed to mount %s at %s", keyVaultObject.ObjectName, targetPath)
		}
		glog.V(0).Infof("Secrets Store csi driver mounted %s", keyVaultObject.ObjectName)
		glog.V(5).Infof("Mount point: %s", targetPath)
	}

	return nil
}

// GetKeyVaultObjectContent get content of the keyvault object
func (p *Provider) GetKeyVaultObjectContent(ctx context.Context, objectType string, objectName string, objectVersion string) (content string, err error) {
	vaultURL, err := p.getVaultURL(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get vault")
	}

	kvClient, err := p.initializeKvClient()
	if err != nil {
		return "", errors.Wrap(err, "failed to get keyvaultClient")
	}

	switch objectType {
	case VaultObjectTypeSecret:
		secret, err := kvClient.GetSecret(ctx, *vaultURL, objectName, objectVersion)
		if err != nil {
			return "", wrapObjectTypeError(err, objectType, objectName, objectVersion)
		}
		return *secret.Value, nil
	case VaultObjectTypeKey:
		keybundle, err := kvClient.GetKey(ctx, *vaultURL, objectName, objectVersion)
		if err != nil {
			return "", wrapObjectTypeError(err, objectType, objectName, objectVersion)
		}
		// NOTE: we are writing the RSA modulus content of the key
		return *keybundle.Key.N, nil
	case VaultObjectTypeCertificate:
		certbundle, err := kvClient.GetCertificate(ctx, *vaultURL, objectName, objectVersion)
		if err != nil {
			return "", wrapObjectTypeError(err, objectType, objectName, objectVersion)
		}
		return string(*certbundle.Cer), nil
	default:
		err := errors.Errorf("Invalid vaultObjectTypes. Should be secret, key, or cert")
		return "", wrapObjectTypeError(err, objectType, objectName, objectVersion)
	}
}

func wrapObjectTypeError(err error, objectType string, objectName string, objectVersion string) error {
	return errors.Wrapf(err, "failed to get objectType:%s, objectName:%s, objectVersion:%s", objectType, objectName, objectVersion)
}
