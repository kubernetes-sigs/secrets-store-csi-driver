// +build e2e

package e2eprovider

// MockSecretsStoreObject holds mock object related config
type MockSecretsStoreObject struct {
	// the name of the secret objects
	ObjectName string `json:"objectName" yaml:"objectName"`
	// the version of the secret objects
	ObjectVersion string `json:"objectVersion" yaml:"objectVersion"`
}

// StringArray holds a list of objects
type StringArray struct {
	Array []string `json:"array" yaml:"array"`
}
