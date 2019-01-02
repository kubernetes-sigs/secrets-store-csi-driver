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
	"fmt"
	"encoding/json"
	"regexp"
	"net/http"
	"os"
	"path"
	"io/ioutil"
	"strings"
	"strconv"
	"golang.org/x/net/context"
	yaml "gopkg.in/yaml.v2"

	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	kv "github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	kvmgmt "github.com/Azure/azure-sdk-for-go/services/keyvault/mgmt/2016-10-01/keyvault"

	"github.com/pkg/errors"
	"github.com/golang/glog"
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
	nmiendpoint         = "http://localhost:2579/host/token/"
	// Pod Identity podnameheader
	podnameheader       = "podname"
	// Pod Identity podnsheader
	podnsheader         = "podns"
)
type NMIResponse struct {
    Token adal.Token `json:"token"`
    ClientID string `json:"clientid"`
}
// OAuthGrantType specifies which grant type to use.
type OAuthGrantType int

func AuthGrantType() OAuthGrantType {
	return OAuthGrantTypeServicePrincipal
}

type AzureProvider struct {
	// the name of the Azure Key Vault instance
	KeyvaultName string
	// the name of the Azure Key Vault objects, since attributes can only be strings, this will be mapped to StringArray, which is an array of KeyVaultObject
	Objects []KeyVaultObject
	// the resourcegroup of the Azure Key Vault
	ResourceGroup string
	// subscriptionId to azure
	SubscriptionId string
	// tenantID in AAD
	TenantId string
	// POD AAD Identity flag
	UsePodIdentity bool
}

type KeyVaultObject struct {
	// the name of the Azure Key Vault objects
	ObjectName string `json:"objectName" yaml:"objectName"`
	// the version of the Azure Key Vault objects
	ObjectVersion string `json:"objectVersion" yaml:"objectVersion"`
	// the type of the Azure Key Vault objects
	ObjectType string `json:"objectType" yaml:"objectType"`
}

type StringArray struct {
	Array []string `json:"array" yaml:"array"`
}

// NewProvider creates a new ACIProvider.
func NewAzureProvider() (*AzureProvider, error) {
	glog.V(2).Infof("NewAzureProvider")
	var p AzureProvider
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

func (p *AzureProvider) GetKeyvaultToken(grantType OAuthGrantType, cloudName string, aADClientSecret string, aADClientID string, podname string, podns string) (authorizer autorest.Authorizer, err error) {
	env, err := ParseAzureEnvironment(cloudName)
	if err != nil {
		return nil, err
	}

	kvEndPoint := env.KeyVaultEndpoint
	if '/' == kvEndPoint[len(kvEndPoint)-1] {
		kvEndPoint = kvEndPoint[:len(kvEndPoint)-1]
	}
	servicePrincipalToken, err := p.GetServicePrincipalToken(env, kvEndPoint, aADClientSecret, aADClientID, podname, podns)
	if err != nil {
		return nil, err
	}
	authorizer = autorest.NewBearerAuthorizer(servicePrincipalToken)
	return authorizer, nil
}

func (p *AzureProvider) initializeKvClient(cloudName string, aADClientSecret string, aADClientID string, podname string, podns string) (*kv.BaseClient, error) {
	kvClient := kv.New()
	token, err := p.GetKeyvaultToken(AuthGrantType(), cloudName, aADClientSecret, aADClientID, podname, podns)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get key vault token")
	}

	kvClient.Authorizer = token
	return &kvClient, nil
}

func GetCredential(secrets map[string]string) (string, string, error) {
	if secrets == nil {
		return "", "", fmt.Errorf("unexpected: getCredential secrets is nil")
	}

	var clientId, clientSecret string
	for k, v := range secrets {
		switch strings.ToLower(k) {
		case "clientid":
			clientId = v
		case "clientsecret":
			clientSecret = v
		}
	}

	if clientId == "" {
		return "", "", fmt.Errorf("could not find clientid in secrets(%v)", secrets)
	}
	if clientSecret == "" {
		return "", "", fmt.Errorf("could not find clientsecret in secrets(%v)", secrets)
	}

	return clientId, clientSecret, nil
}

func (p *AzureProvider) getVaultURL(ctx context.Context, cloudName string, aADClientSecret string, aADClientID string, podName string, podns string) (vaultUrl *string, err error) {
	glog.V(5).Infof("subscriptionID: %s", p.SubscriptionId)
	glog.V(5).Infof("vaultName: %s", p.KeyvaultName)
	glog.V(5).Infof("resourceGroup: %s", p.ResourceGroup)

	vaultsClient := kvmgmt.NewVaultsClient(p.SubscriptionId)
	token, tokenErr := p.GetManagementToken(AuthGrantType(),
		cloudName,
		aADClientSecret,
		aADClientID,
		podName,
		podns)
	if tokenErr != nil {
		return nil, errors.Wrapf(err, "failed to get management token")
	}
	vaultsClient.Authorizer = token
	vault, err := vaultsClient.Get(ctx, p.ResourceGroup, p.KeyvaultName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get vault %s", p.KeyvaultName)
	}
	return vault.Properties.VaultURI, nil
}

func (p *AzureProvider) GetManagementToken(grantType OAuthGrantType, cloudName string, aADClientSecret string, aADClientID string, podname string, podns string) (authorizer autorest.Authorizer, err error) {
	
	env, err := ParseAzureEnvironment(cloudName)
	if err != nil {
		return nil, err
	}

	rmEndPoint := env.ResourceManagerEndpoint
	servicePrincipalToken, err := p.GetServicePrincipalToken(env, rmEndPoint, aADClientSecret, aADClientID, podname, podns)
	if err != nil {
		return nil, err
	}
	authorizer = autorest.NewBearerAuthorizer(servicePrincipalToken)
	return authorizer, nil
}

// GetServicePrincipalToken creates a new service principal token based on the configuration
func (p *AzureProvider) GetServicePrincipalToken(env *azure.Environment, resource string, aADClientSecret string, aADClientID string, podname string, podns string) (*adal.ServicePrincipalToken, error) {
	oauthConfig, err := adal.NewOAuthConfig(env.ActiveDirectoryEndpoint, p.TenantId)
	if err != nil {
		return nil, fmt.Errorf("creating the OAuth config: %v", err)
	}

	// For usepodidentity mode, the CSI driver makes an authorization request to fetch token for a resource from the NMI host endpoint (http://127.0.0.1:2579/host/token/). 
	// The request includes the pod namespace `podns` and the pod name `podname` in the request header and the resource endpoint of the resource requesting the token. 
	// The NMI server identifies the pod based on the `podns` and `podname` in the request header and then queries k8s (through MIC) for a matching azure identity.  
	// Then nmi makes an adal request to get a token for the resource in the request, returns the `token` and the `clientid` as a reponse to the CSI request.

	if p.UsePodIdentity {
		glog.V(0).Infoln("azure: using pod identity to retrieve token")
		
		endpoint := fmt.Sprintf("%s?resource=%s", nmiendpoint, resource)
		client := &http.Client{}
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Add(podnsheader, podns)
		req.Header.Add(podnameheader, podname)
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
			
			r, _ := regexp.Compile("^(\\S{4})(\\S|\\s)*(\\S{4})$")
			fmt.Printf("\n accesstoken: %s\n", r.ReplaceAllString(nmiResp.Token.AccessToken, "$1##### REDACTED #####$3"))
			fmt.Printf("\n clientid: %s\n", r.ReplaceAllString(nmiResp.ClientID, "$1##### REDACTED #####$3"))

			token := nmiResp.Token
			clientID := nmiResp.ClientID

			if &token == nil || clientID == "" {
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
	if len(aADClientSecret) > 0 {
		glog.V(2).Infoln("azure: using client_id+client_secret to retrieve access token")
		return adal.NewServicePrincipalToken(
			*oauthConfig,
			aADClientID,
			aADClientSecret,
			resource)
	}

	return nil, fmt.Errorf("No credentials provided for AAD application %s", aADClientID)
}
// MountKeyVaultObjectContent mounts content of the keyvault object to target path
func (p *AzureProvider) MountKeyVaultObjectContent(ctx context.Context, attrib map[string]string, secrets map[string]string, targetPath string, permission os.FileMode) (err error) {
	keyvaultName := attrib["keyvaultName"]
	usePodIdentity, err := strconv.ParseBool(attrib["usePodIdentity"])
	if err != nil {
		fmt.Printf("cannot parse usePodIdentity")
		return err
	}
	resourceGroup := attrib["resourceGroup"]
	subscriptionId := attrib["subscriptionId"]
	tenantId := attrib["tenantId"]

	keyVaultObjects := []KeyVaultObject{}
	objectsStrings := attrib["objects"]
	fmt.Printf("objectsStrings: [%s]", objectsStrings)

	var objects StringArray
	err = yaml.Unmarshal([]byte(objectsStrings), &objects)
	if err != nil {
		fmt.Printf("unmarshall failed for objects")
		return err
	}
	fmt.Printf("objects: [%v]", objects.Array)
	for _, object := range objects.Array {
		fmt.Printf("unmarshal object: [%s]", object)
		var keyVaultObject KeyVaultObject
		err = yaml.Unmarshal([]byte(object), &keyVaultObject)
		if err != nil {
			fmt.Printf("unmarshall failed for keyVaultObjects at index")
			return err
		}
		keyVaultObjects = append(keyVaultObjects, keyVaultObject)
	}
	
	fmt.Printf("unmarshalled property: %v", keyVaultObjects)
	fmt.Printf("keyVaultObjects len: %d", len(keyVaultObjects))

	var clientId, clientSecret string

	if !usePodIdentity {
		glog.V(0).Infoln("using pod identity to access keyvault")
		clientId, clientSecret, err = GetCredential(secrets)
		if err != nil {
			return err
		}
	}
	p.KeyvaultName = keyvaultName
	p.UsePodIdentity = usePodIdentity
	p.ResourceGroup = resourceGroup
	p.SubscriptionId = subscriptionId
	p.TenantId = tenantId

	for _, keyVaultObject := range keyVaultObjects {
		content, err := p.GetKeyVaultObjectContent(ctx, keyVaultObject.ObjectType, keyVaultObject.ObjectName, keyVaultObject.ObjectVersion, clientId, clientSecret)
		if err != nil {
			return err
		}
		objectContent := []byte(content)
		if err := ioutil.WriteFile(path.Join(targetPath, keyVaultObject.ObjectName), objectContent, permission); err != nil {
			return errors.Wrapf(err, "KeyVault failed to write %s at %s", keyVaultObject.ObjectName, targetPath)
		}
		glog.V(0).Infof("KeyVault wrote %s at %s",keyVaultObject.ObjectName, targetPath)
	}
	
	return nil
}
// GetKeyVaultObjectContent get content of the keyvault object
func (p *AzureProvider) GetKeyVaultObjectContent(ctx context.Context, objectType string, objectName string, objectVersion string, clientId string, clientSecret string) (content string, err error) {
	// TODO: support pod identity

	vaultUrl, err := p.getVaultURL(ctx, "", clientSecret, clientId, "", "")
	if err != nil {
		return "", errors.Wrap(err, "failed to get vault")
	}

	kvClient, err := p.initializeKvClient("", clientSecret, clientId, "", "")
	if err != nil {
		return "", errors.Wrap(err, "failed to get keyvaultClient")
	}

	switch objectType {
	case VaultObjectTypeSecret:
		secret, err := kvClient.GetSecret(ctx, *vaultUrl, objectName, objectVersion)
		if err != nil {
			return "", wrapObjectTypeError(err, objectType, objectName, objectVersion)
		}
		return *secret.Value, nil
	case VaultObjectTypeKey:
		keybundle, err := kvClient.GetKey(ctx, *vaultUrl, objectName, objectVersion)
		if err != nil {
			return "", wrapObjectTypeError(err, objectType, objectName, objectVersion)
		}
		// NOTE: we are writing the RSA modulus content of the key
		return *keybundle.Key.N, nil
	case VaultObjectTypeCertificate:
		certbundle, err := kvClient.GetCertificate(ctx, *vaultUrl, objectName, objectVersion)
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
	return errors.Wrapf(err, "failed to get objectType:%s, objetName:%s, objectVersion:%s", objectType, objectName, objectVersion)
}