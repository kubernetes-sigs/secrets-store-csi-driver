/*
Copyright 2020 The Kubernetes Authors.

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
	"bytes"
	"context"
	"io/ioutil"
	"path/filepath"
	"text/template"

	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/secrets-store-csi-driver/test/e2e/framework"
)

const (
	azureSecretsStoreCredsFile = "azure/secrets-store-creds.yaml"
)

type SetupAzureInput struct {
	Creator        framework.Creator
	GetLister      framework.GetLister
	Namespace      string
	ManifestsDir   string
	KubeconfigPath string
	ClientID       string
	ClientSecret   string
}

func SetupAzure(ctx context.Context, input SetupAzureInput) {
	installSecretsStoreCredential(ctx, input)
}

func installSecretsStoreCredential(ctx context.Context, input SetupAzureInput) {
	framework.Byf("%s: Installing credential service account", input.Namespace)

	data, err := ioutil.ReadFile(filepath.Join(input.ManifestsDir, azureSecretsStoreCredsFile))
	Expect(err).To(Succeed())

	buf := new(bytes.Buffer)
	err = template.Must(template.New("").Parse(string(data))).Execute(buf, struct {
		Namespace    string
		ClientID     string
		ClientSecret string
	}{
		input.Namespace,
		input.ClientID,
		input.ClientSecret,
	})
	Expect(err).To(Succeed())

	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, nil)
	Expect(err).To(Succeed())

	Expect(input.Creator.Create(ctx, obj)).To(Succeed())
}

type TeardownAzureInput struct {
	Deleter        framework.Deleter
	GetLister      framework.GetLister
	Namespace      string
	ManifestsDir   string
	KubeconfigPath string
}

func TeardownAzure(ctx context.Context, input TeardownAzureInput) {
	// Delete only cluster-wide resources, other resources are deleted by deleting namespace
}
