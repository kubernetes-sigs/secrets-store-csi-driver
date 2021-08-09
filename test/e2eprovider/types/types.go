package types

// KeyVaultObject holds keyvault object related config
type KeyVaultObject struct {
	// the name of the Azure Key Vault objects
	ObjectName string `json:"objectName" yaml:"objectName"`
	// the filename the object will be written to
	ObjectAlias string `json:"objectAlias" yaml:"objectAlias"`
	// the version of the Azure Key Vault objects
	ObjectVersion string `json:"objectVersion" yaml:"objectVersion"`
	// the type of the Azure Key Vault objects
	ObjectType string `json:"objectType" yaml:"objectType"`
	// the format of the Azure Key Vault objects
	// supported formats are PEM, PFX
	ObjectFormat string `json:"objectFormat" yaml:"objectFormat"`
	// The encoding of the object in KeyVault
	// Supported encodings are Base64, Hex, Utf-8
	ObjectEncoding string `json:"objectEncoding" yaml:"objectEncoding"`
}

// StringArray holds a list of objects
type StringArray struct {
	Array []string `json:"array" yaml:"array"`
}
