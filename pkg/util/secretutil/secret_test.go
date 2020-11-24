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

package secretutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/secrets-store-csi-driver/apis/v1alpha1"

	"github.com/stretchr/testify/assert"
)

const (
	certFile = `
-----BEGIN CERTIFICATE-----
MIIDOTCCAiGgAwIBAgIJAP0J5Z7N0Y5fMA0GCSqGSIb3DQEBCwUAMDMxFzAVBgNV
BAMMDmRlbW8uYXp1cmUuY29tMRgwFgYDVQQKDA9ha3MtaW5ncmVzcy10bHMwHhcN
MjAwNDE1MDQyMzQ2WhcNMjEwNDE1MDQyMzQ2WjAzMRcwFQYDVQQDDA5kZW1vLmF6
dXJlLmNvbTEYMBYGA1UECgwPYWtzLWluZ3Jlc3MtdGxzMIIBIjANBgkqhkiG9w0B
AQEFAAOCAQ8AMIIBCgKCAQEAyS3Zky3n8JlLBxPLzgUpKZYxvzRadeWLmWVbK9by
o08S0Ss8Jao7Ay1wHtnLbn52rzCX6IX1sAe1TAT755Gk7JtLMkshtj6F8BNeelEy
E1gsBE5ntY5vyLTm/jZUIKz2Z9TLnqvQTmp6gJ68BKJ1NobnsHiAcKc6hI7kmY9C
oshmAi5qiKYBgzv/thji0093vtVSa9iwHhQp+AEIMhkvM5ZZkiU5eE6MT9SBEcVW
KmWF28UsB04daYwS2MKJ5l6d4n0LUdAG0FBt1lCoT9rwUDj9l3Mqmi953gw26LUr
NrYnM/8N2jl7Cuyw5alIWaUDrt5i+pu8wdWfzVk+fO7x8QIDAQABo1AwTjAdBgNV
HQ4EFgQUwFBbR014McETdrGGklpEQcl71Q0wHwYDVR0jBBgwFoAUwFBbR014McET
drGGklpEQcl71Q0wDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEATgTy
gg1Q6ISSekiBCe12dqUTMFQh9GKpfYWKRbMtjOjpc7Mdwkdmm3Fu6l3RfEFT28Ij
fy97LMYv8W7beemDFqdmneb2w2ww0ZAFJg+GqIJZ9s/JadiFBDNU7CmJMhA225Qz
XC8ovejiePslnL4QJWlhVG93ZlBJ6SDkRgfcoIW2x4IBE6wv7jmRF4lOvb3z1ddP
iPQqhbEEbwMpXmWv7/2RnjAHdjdGaWRMC5+CaI+lqHyj6ir1c+e6u1QUY54qjmgM
koN/frqYab5Ek3kauj1iqW7rPkrFCqT2evh0YRqb1bFsCLJrRNxnOZ5wKXV/OYQa
QX5t0wFGCZ0KlbXDiw==
-----END CERTIFICATE-----
-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDJLdmTLefwmUsH
E8vOBSkpljG/NFp15YuZZVsr1vKjTxLRKzwlqjsDLXAe2ctufnavMJfohfWwB7VM
BPvnkaTsm0sySyG2PoXwE156UTITWCwETme1jm/ItOb+NlQgrPZn1Mueq9BOanqA
nrwEonU2hueweIBwpzqEjuSZj0KiyGYCLmqIpgGDO/+2GOLTT3e+1VJr2LAeFCn4
AQgyGS8zllmSJTl4ToxP1IERxVYqZYXbxSwHTh1pjBLYwonmXp3ifQtR0AbQUG3W
UKhP2vBQOP2XcyqaL3neDDbotSs2ticz/w3aOXsK7LDlqUhZpQOu3mL6m7zB1Z/N
WT587vHxAgMBAAECggEAJb0qIYftCJ9ZCbzW8JDbRefc8SdbCN7Er0PqNHEgFy6Q
MxjPMambZF8ztzXYCaRDk12kQYRPsHPhuJ7+ulQCAjinhIm/izZzXbPkd0GgCSzz
JOOoZNCRe68j3fBHG9IWbyfmAp/sdalXzaT5VE09e7sW323bekaEnbVIgN30/CAS
gI77YdaIhG+PT/pSCOc11MTkBJp+VhT1tEtlRAR78b1RXbGi1oUHRee7C3Ia8IKQ
3L5dPxR9RsYsR2O66908kEi8ZcuIjcbIuRPDXYHY+5Nwm3mXuZlkyjyfxJXsIA8i
qBrQrSpHGgAn1TVlLDSCKPLbkRzBRRvAW0zL/cDTuQKBgQDq/9Yxx9QivAuUxxdE
u0VO5CzzZYFWhDxAXS3/wYyo1YnoPtUz/lGCvMWp0k2aaa0+KTXv2fRCUGSujHW7
Jfo4kuMPkauAhoXx9QJAcjoK0nNbYEaqoJyMoRID+Qb9XHkj+lmBTmMVgALCT9DI
HekHj/M3b7CknbfWv1sOZ/vpQwKBgQDbKEuP/DWQa9DC5nn5phHD/LWZLG/cMR4X
TmwM/cbfRxM/6W0+/KLAodz4amGRzVlW6ax4k26BSE8Zt/SiyA1DQRTeFloduoqW
iWF4dMeItxw2am+xLREwtoN3FgsJHu2z/O/0aaBAOMLUXIPIyiE4L6OnEPifE/pb
AM8EbM5auwKBgGhdABIRjbtzSa1kEYhbprcXjIL3lE4I4f0vpIsNuNsOInW62dKC
Yk6uaRY3KHGn9uFBSgvf/qMost310R8xCYPwb9htN/4XQAspZTubvv0pY0O0aQ3D
0GJ/8dFD2f/Q/pekyfUsC8Lzm8YRzkXhSqkqG7iF6Kviw08iolyuf2ijAoGBANaA
pRzDvWWisUziKsa3zbGnGdNXVBEPniUvo8A/b7RAK84lWcEJov6qLs6RyPfdJrFT
u3S00LcHICzLCU1+QsTt4U/STtfEKjtXMailnFrq5lk4aiPfOXEVYq1fTOPbesrt
Katu6uOQ6tjRyEbx1/vXXPV7Peztr9/8daMeIAdbAoGBAOYRJ1CzMYQKjWF32Uas
7hhQxyH1QI4nV56Dryq7l/UWun2pfwNLZFqOHD3qm05aznzNKvk9aHAsOPFfUUXO
7sp0Ge5FLMSw1uMNnutcVcMz37KAY2fOoE2xoLM4DU/H2NqDjeGCsOsU1ReRS1vB
J+42JGwBdLV99ruYKVKOWPh4
-----END PRIVATE KEY-----	
`
	certPEM = `-----BEGIN CERTIFICATE-----
MIIDOTCCAiGgAwIBAgIJAP0J5Z7N0Y5fMA0GCSqGSIb3DQEBCwUAMDMxFzAVBgNV
BAMMDmRlbW8uYXp1cmUuY29tMRgwFgYDVQQKDA9ha3MtaW5ncmVzcy10bHMwHhcN
MjAwNDE1MDQyMzQ2WhcNMjEwNDE1MDQyMzQ2WjAzMRcwFQYDVQQDDA5kZW1vLmF6
dXJlLmNvbTEYMBYGA1UECgwPYWtzLWluZ3Jlc3MtdGxzMIIBIjANBgkqhkiG9w0B
AQEFAAOCAQ8AMIIBCgKCAQEAyS3Zky3n8JlLBxPLzgUpKZYxvzRadeWLmWVbK9by
o08S0Ss8Jao7Ay1wHtnLbn52rzCX6IX1sAe1TAT755Gk7JtLMkshtj6F8BNeelEy
E1gsBE5ntY5vyLTm/jZUIKz2Z9TLnqvQTmp6gJ68BKJ1NobnsHiAcKc6hI7kmY9C
oshmAi5qiKYBgzv/thji0093vtVSa9iwHhQp+AEIMhkvM5ZZkiU5eE6MT9SBEcVW
KmWF28UsB04daYwS2MKJ5l6d4n0LUdAG0FBt1lCoT9rwUDj9l3Mqmi953gw26LUr
NrYnM/8N2jl7Cuyw5alIWaUDrt5i+pu8wdWfzVk+fO7x8QIDAQABo1AwTjAdBgNV
HQ4EFgQUwFBbR014McETdrGGklpEQcl71Q0wHwYDVR0jBBgwFoAUwFBbR014McET
drGGklpEQcl71Q0wDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEATgTy
gg1Q6ISSekiBCe12dqUTMFQh9GKpfYWKRbMtjOjpc7Mdwkdmm3Fu6l3RfEFT28Ij
fy97LMYv8W7beemDFqdmneb2w2ww0ZAFJg+GqIJZ9s/JadiFBDNU7CmJMhA225Qz
XC8ovejiePslnL4QJWlhVG93ZlBJ6SDkRgfcoIW2x4IBE6wv7jmRF4lOvb3z1ddP
iPQqhbEEbwMpXmWv7/2RnjAHdjdGaWRMC5+CaI+lqHyj6ir1c+e6u1QUY54qjmgM
koN/frqYab5Ek3kauj1iqW7rPkrFCqT2evh0YRqb1bFsCLJrRNxnOZ5wKXV/OYQa
QX5t0wFGCZ0KlbXDiw==
-----END CERTIFICATE-----
`
	keyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAyS3Zky3n8JlLBxPLzgUpKZYxvzRadeWLmWVbK9byo08S0Ss8
Jao7Ay1wHtnLbn52rzCX6IX1sAe1TAT755Gk7JtLMkshtj6F8BNeelEyE1gsBE5n
tY5vyLTm/jZUIKz2Z9TLnqvQTmp6gJ68BKJ1NobnsHiAcKc6hI7kmY9CoshmAi5q
iKYBgzv/thji0093vtVSa9iwHhQp+AEIMhkvM5ZZkiU5eE6MT9SBEcVWKmWF28Us
B04daYwS2MKJ5l6d4n0LUdAG0FBt1lCoT9rwUDj9l3Mqmi953gw26LUrNrYnM/8N
2jl7Cuyw5alIWaUDrt5i+pu8wdWfzVk+fO7x8QIDAQABAoIBACW9KiGH7QifWQm8
1vCQ20Xn3PEnWwjexK9D6jRxIBcukDMYzzGpm2RfM7c12AmkQ5NdpEGET7Bz4bie
/rpUAgI4p4SJv4s2c12z5HdBoAks8yTjqGTQkXuvI93wRxvSFm8n5gKf7HWpV82k
+VRNPXu7Ft9t23pGhJ21SIDd9PwgEoCO+2HWiIRvj0/6UgjnNdTE5ASaflYU9bRL
ZUQEe/G9UV2xotaFB0XnuwtyGvCCkNy+XT8UfUbGLEdjuuvdPJBIvGXLiI3GyLkT
w12B2PuTcJt5l7mZZMo8n8SV7CAPIqga0K0qRxoAJ9U1ZSw0gijy25EcwUUbwFtM
y/3A07kCgYEA6v/WMcfUIrwLlMcXRLtFTuQs82WBVoQ8QF0t/8GMqNWJ6D7VM/5R
grzFqdJNmmmtPik179n0QlBkrox1uyX6OJLjD5GrgIaF8fUCQHI6CtJzW2BGqqCc
jKESA/kG/Vx5I/pZgU5jFYACwk/QyB3pB4/zN2+wpJ231r9bDmf76UMCgYEA2yhL
j/w1kGvQwuZ5+aYRw/y1mSxv3DEeF05sDP3G30cTP+ltPvyiwKHc+Gphkc1ZVums
eJNugUhPGbf0osgNQ0EU3hZaHbqKlolheHTHiLccNmpvsS0RMLaDdxYLCR7ts/zv
9GmgQDjC1FyDyMohOC+jpxD4nxP6WwDPBGzOWrsCgYBoXQASEY27c0mtZBGIW6a3
F4yC95ROCOH9L6SLDbjbDiJ1utnSgmJOrmkWNyhxp/bhQUoL3/6jKLLd9dEfMQmD
8G/YbTf+F0ALKWU7m779KWNDtGkNw9Bif/HRQ9n/0P6XpMn1LAvC85vGEc5F4Uqp
Khu4heir4sNPIqJcrn9oowKBgQDWgKUcw71lorFM4irGt82xpxnTV1QRD54lL6PA
P2+0QCvOJVnBCaL+qi7Okcj33SaxU7t0tNC3ByAsywlNfkLE7eFP0k7XxCo7VzGo
pZxa6uZZOGoj3zlxFWKtX0zj23rK7SmrburjkOrY0chG8df711z1ez3s7a/f/HWj
HiAHWwKBgQDmESdQszGECo1hd9lGrO4YUMch9UCOJ1eeg68qu5f1Frp9qX8DS2Ra
jhw96ptOWs58zSr5PWhwLDjxX1FFzu7KdBnuRSzEsNbjDZ7rXFXDM9+ygGNnzqBN
saCzOA1Px9jag43hgrDrFNUXkUtbwSfuNiRsAXS1ffa7mClSjlj4eA==
-----END RSA PRIVATE KEY-----
`
)

func TestGetCert(t *testing.T) {
	cases := []struct {
		Name        string
		data        string
		part        string
		expectedPEM []byte
		expectedErr bool
	}{
		{
			Name:        "Get cert PEM",
			data:        certFile,
			part:        "tls.crt",
			expectedPEM: []byte(certPEM),
			expectedErr: false,
		},
		{
			Name:        "Get key PEM",
			data:        certFile,
			part:        "tls.key",
			expectedPEM: []byte(keyPEM),
			expectedErr: false,
		},
		{
			Name:        "Unsupported part type",
			data:        certFile,
			part:        "key",
			expectedPEM: []byte(nil),
			expectedErr: true,
		},
	}

	for _, tc := range cases {
		actualPEM, err := GetCertPart([]byte(tc.data), tc.part)
		assert.Equal(t, tc.expectedErr, err != nil)
		assert.Equal(t, tc.expectedPEM, actualPEM)
	}
}

func TestValidateSecretObject(t *testing.T) {
	tests := []struct {
		name          string
		secretObj     v1alpha1.SecretObject
		expectedError bool
	}{
		{
			name:          "secret name is empty",
			secretObj:     v1alpha1.SecretObject{},
			expectedError: true,
		},
		{
			name:          "secret type is empty",
			secretObj:     v1alpha1.SecretObject{SecretName: "secret1"},
			expectedError: true,
		},
		{
			name:          "data is empty",
			secretObj:     v1alpha1.SecretObject{SecretName: "secret1", Type: "Opaque"},
			expectedError: true,
		},
		{
			name: "valid secret object",
			secretObj: v1alpha1.SecretObject{
				SecretName: "secret1",
				Type:       "Opaque",
				Data:       []*v1alpha1.SecretObjectData{{ObjectName: "obj1", Key: "file1"}}},
			expectedError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateSecretObject(test.secretObj)
			if test.expectedError && err == nil {
				t.Fatalf("expected err: %+v, got: %+v", test.expectedError, err)
			}
		})
	}
}

func TestGetSecretData(t *testing.T) {
	tests := []struct {
		name            string
		secretObjData   []*v1alpha1.SecretObjectData
		secretType      corev1.SecretType
		currentFiles    map[string]string
		expectedDataMap map[string][]byte
		expectedError   bool
	}{
		{
			name: "object name not set",
			secretObjData: []*v1alpha1.SecretObjectData{
				{
					ObjectName: "",
				},
			},
			secretType:      corev1.SecretTypeOpaque,
			expectedDataMap: make(map[string][]byte),
			expectedError:   true,
		},
		{
			name: "key not set",
			secretObjData: []*v1alpha1.SecretObjectData{
				{
					ObjectName: "obj1",
				},
			},
			secretType:      corev1.SecretTypeOpaque,
			expectedDataMap: make(map[string][]byte),
			expectedError:   true,
		},
		{
			name: "file matching object doesn't exist in map",
			secretObjData: []*v1alpha1.SecretObjectData{
				{
					ObjectName: "obj1",
					Key:        "file1",
				},
			},
			secretType:      corev1.SecretTypeOpaque,
			currentFiles:    map[string]string{"obj2": ""},
			expectedDataMap: make(map[string][]byte),
			expectedError:   true,
		},
		{
			name: "file matching object doesn't exist in the fs",
			secretObjData: []*v1alpha1.SecretObjectData{
				{
					ObjectName: "obj1",
					Key:        "file1",
				},
			},
			secretType:      corev1.SecretTypeOpaque,
			currentFiles:    map[string]string{"obj2": ""},
			expectedDataMap: make(map[string][]byte),
			expectedError:   true,
		},
		{
			name: "file matching object found in fs",
			secretObjData: []*v1alpha1.SecretObjectData{
				{
					ObjectName: "obj1",
					Key:        "file1",
				},
			},
			secretType:      corev1.SecretTypeOpaque,
			currentFiles:    map[string]string{"obj1": ""},
			expectedDataMap: map[string][]byte{"file1": []byte("test")},
			expectedError:   false,
		},
		{
			name: "file matching object found in fs after trimming spaces in object name",
			secretObjData: []*v1alpha1.SecretObjectData{
				{
					ObjectName: "obj1     ",
					Key:        "file1",
				},
			},
			secretType:      corev1.SecretTypeOpaque,
			currentFiles:    map[string]string{"obj1": ""},
			expectedDataMap: map[string][]byte{"file1": []byte("test")},
			expectedError:   false,
		},
		{
			name: "file matching object found in fs after trimming spaces in key",
			secretObjData: []*v1alpha1.SecretObjectData{
				{
					ObjectName: "obj1     ",
					Key:        "   file1",
				},
			},
			secretType:      corev1.SecretTypeOpaque,
			currentFiles:    map[string]string{"obj1": ""},
			expectedDataMap: map[string][]byte{"file1": []byte("test")},
			expectedError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tmpDir, err := ioutil.TempDir("", "ut")
			if err != nil {
				t.Fatalf("expected err to be nil, got: %+v", err)
			}
			defer os.RemoveAll(tmpDir)

			for fileName := range test.currentFiles {
				filePath, err := createTestFile(tmpDir, fileName)
				if err != nil {
					t.Fatalf("expected err to be nil, got: %+v", err)
				}
				test.currentFiles[fileName] = filePath
			}
			datamap, err := GetSecretData(test.secretObjData, test.secretType, test.currentFiles)
			if test.expectedError && err == nil {
				t.Fatalf("expected err: %+v, got: %+v", test.expectedError, err)
			}
			if !reflect.DeepEqual(datamap, test.expectedDataMap) {
				t.Fatalf("expected data map doesn't match actual")
			}
		})
	}
}

func createTestFile(tmpDir, fileName string) (string, error) {
	if fileName != "" {
		filePath := fmt.Sprintf("%s/%s", tmpDir, fileName)
		f, err := os.Create(filePath)
		f.Write([]byte("test"))
		defer f.Close()
		return filePath, err
	}
	return "", nil
}

func TestGenerateSHAFromSecret(t *testing.T) {
	tests := []struct {
		name             string
		data1            map[string][]byte
		data2            map[string][]byte
		expectedSHAMatch bool
	}{
		{
			name:             "SHA mismatch as data1 missing key",
			data1:            map[string][]byte{},
			data2:            map[string][]byte{"key": []byte("value")},
			expectedSHAMatch: false,
		},
		{
			name:             "SHA mismatch as data1 key different",
			data1:            map[string][]byte{"key": []byte("oldvalue")},
			data2:            map[string][]byte{"key": []byte("newvalue")},
			expectedSHAMatch: false,
		},
		{
			name:             "SHA match for multiple data keys in different order",
			data1:            map[string][]byte{"key1": []byte("value1"), "key2": []byte("value2")},
			data2:            map[string][]byte{"key2": []byte("value2"), "key1": []byte("value1")},
			expectedSHAMatch: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sha1, err := GetSHAFromSecret(test.data1)
			assert.NoError(t, err)

			sha2, err := GetSHAFromSecret(test.data2)
			assert.NoError(t, err)

			assert.Equal(t, test.expectedSHAMatch, sha1 == sha2)
		})
	}
}

func TestGetPrivateKey(t *testing.T) {
	tests := []struct {
		name        string
		actualPEM   string
		expectedKey string
		expectedErr bool
	}{
		{
			name: "RSA Key",
			actualPEM: `
-----BEGIN CERTIFICATE-----
MIIC5DCCAcwCCQClrnRsmeWS4TANBgkqhkiG9w0BAQsFADA0MRYwFAYDVQQDDA1k
ZW1vLnRlc3QuY29tMRowGAYDVQQKDBFpbmdyZXNzLXRscy1jZXJ0MTAeFw0yMDEw
MDgxOTAwMDJaFw0yMTEwMDgxOTAwMDJaMDQxFjAUBgNVBAMMDWRlbW8udGVzdC5j
b20xGjAYBgNVBAoMEWluZ3Jlc3MtdGxzLWNlcnQxMIIBIjANBgkqhkiG9w0BAQEF
AAOCAQ8AMIIBCgKCAQEA0AWQCdeukwkzIKKJNp3DaRe9azBZ8J/NFb2Nczq3Y8xc
MDB/eT7lfMMNYluLQPDzkRN9QHKiz8ei9ynxRiEC/Al2OsdZPdPqNxnBVDsFcD72
9nofroBUXRch5dP5amXu5gP628Yu7l8pBoV+lOyyDGkRVHPecegxiVbxtjqhlrwl
hRRFzFGat1CiDq03Gtz1xH/pgaFQzKbTZ1rQE8JcTryZaTYfo5PrUDwhv8PfVHoH
MEqpN54onSoA2JLBeZz7xJvL6pBg0c6OhNCnUYEZBDnyHDBBJJ6FUijKQp6mZNbe
di6Ih4QGJYeLP4HaJdPf9aXlChnbbwEaeBeedXzPjwIDAQABMA0GCSqGSIb3DQEB
CwUAA4IBAQC3NVwO2MxISN9dwXlUUPnGpI2EIEmleDaN1hE28RN+GwYqUZvfg8FQ
HV+qYtc3gHoFdcVeQjTQHNJ7u+4U6PGNQj/UoKd6RY7AEMly4kQq2LtfMZDQYlvt
/xtDDxw1esEgv5P+uXb2ICRnO3p7cOt6/EAK83uYBmpy/FwgNIjJATcm6GmKMRZ6
y0UsfOws9yCOgSdtmp8tWduZL56e8yZ/+gCUMiGDr1f/m0th/zgEvxyIYY3kVh6c
z96TlWVQU9TCYIMg0rBRsPuJcJF7fedQbIRUP5t+cghu7OpbiDDzlBBjAPVhrC2M
FMtqlqaKfhLwz3SzIu8Wcj//cbm6KXLZ
-----END CERTIFICATE-----
-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDQBZAJ166TCTMg
ook2ncNpF71rMFnwn80VvY1zOrdjzFwwMH95PuV8ww1iW4tA8PORE31AcqLPx6L3
KfFGIQL8CXY6x1k90+o3GcFUOwVwPvb2eh+ugFRdFyHl0/lqZe7mA/rbxi7uXykG
hX6U7LIMaRFUc95x6DGJVvG2OqGWvCWFFEXMUZq3UKIOrTca3PXEf+mBoVDMptNn
WtATwlxOvJlpNh+jk+tQPCG/w99UegcwSqk3niidKgDYksF5nPvEm8vqkGDRzo6E
0KdRgRkEOfIcMEEknoVSKMpCnqZk1t52LoiHhAYlh4s/gdol09/1peUKGdtvARp4
F551fM+PAgMBAAECggEBAMxTunDAhvxsO+khXa/k9M1kgS0pOB7PiE2De84kbYA8
eoznBj8c1aNfn+Tt0HGAe24T+6JzN5LqIBuw+goNYPYZgSUpLHI7lkJ7LNfEhYoE
fuYJfNcVvEgX8bbjKIknCKqsXBrFptGDbTO3qmczu4vPJDOVAHlYPlgNq6x4GMKJ
05v1GL3as2db6D8fphm0jdt4QCD+BMP+s/nm5xGOnquZvBn3RUDw7x+tilXuh9fG
l6S8PVDxWuTdfAG5urTW2DtrxSBqXjgClo5ft79frHDpvAhJ7XMIKbVgo+M0quGp
wTi6McCCFVtJP6xv1eI2TRO8xvWoX92H7PHuIJqWrFkCgYEA+M015rLmECmahB2L
LJ8/BH9HMAf15JqbxafmknNDPacsUZujOad87mO8jToAK6aBLwtmIgaYGVs+spC0
v3VnV+3AqAEYKCoj0GmyQcM/Thn9A0OVE0CDPeq0A1OYqXr1G8G/zZDIvOxbBwsm
FXGAxOw0+d3hnuIH2ygHaYbSIU0CgYEA1gpPMO/AzqgKa1GffzOCtf7qNzam0IC5
Bh4vumfnVNuWNw/ReQnwuQVoEreXMbU1SEsOA5wRsUS1mnCliANiVtXDK3ebdBRA
Oqb3cnzql/UnWNYXzU9iBQlpLv/yUHMNSIr49nhdXrNgEXFQLLbKHmvGzKEGjEtX
ShEP7BsaRksCgYEAzSLNhVgNjlfvGW0Oeg0WtUuH01dM6156fv6PgkJct3GlfefY
LcolnJxJMxwWVecj7jj0zasoLwfnau0ayh0vxvS1ew/j7gHIo6byHXyxLmEJFm7b
dBMl4qAoKfH8FgjWHTujPAdbK0GpT+ZmURnTdQnYKAhEZW6x0YVwjxZlHKUCgYBI
zETW7hRztS+mBKLszoY8hDEBCnN+IunLLOUqz0Ac2nqiy5yBQGJBa5dUFmE0JN+0
cOKZU7GoyyfBGWMTeaMuyZGR7SJQPrsBt9wdcmMPv+/cBSUfTUqXT/YYaDDwL9Fq
xOmcWp/XH8ci55lPO/ROmHWLD5F8kftkU51IvocXNQKBgGmh32aF2WOHhWzKxmp4
V9uWIRJv657s9Vlv/5f2UnsMBMirj99quGL1iSSdEComYoRyyiaflvfkqPRAHCIN
0QTu0hJ2SPfqOChrPqnLK6P3KzUGUI3R8EfZAkYWkndMEqoijaIaY8ctdlUVqM8X
8o1UNU2Vz0RQitpWCZbAO5nu
-----END PRIVATE KEY-----
`,
			expectedKey: `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0AWQCdeukwkzIKKJNp3DaRe9azBZ8J/NFb2Nczq3Y8xcMDB/
eT7lfMMNYluLQPDzkRN9QHKiz8ei9ynxRiEC/Al2OsdZPdPqNxnBVDsFcD729nof
roBUXRch5dP5amXu5gP628Yu7l8pBoV+lOyyDGkRVHPecegxiVbxtjqhlrwlhRRF
zFGat1CiDq03Gtz1xH/pgaFQzKbTZ1rQE8JcTryZaTYfo5PrUDwhv8PfVHoHMEqp
N54onSoA2JLBeZz7xJvL6pBg0c6OhNCnUYEZBDnyHDBBJJ6FUijKQp6mZNbedi6I
h4QGJYeLP4HaJdPf9aXlChnbbwEaeBeedXzPjwIDAQABAoIBAQDMU7pwwIb8bDvp
IV2v5PTNZIEtKTgez4hNg3vOJG2APHqM5wY/HNWjX5/k7dBxgHtuE/uiczeS6iAb
sPoKDWD2GYElKSxyO5ZCeyzXxIWKBH7mCXzXFbxIF/G24yiJJwiqrFwaxabRg20z
t6pnM7uLzyQzlQB5WD5YDauseBjCidOb9Ri92rNnW+g/H6YZtI3beEAg/gTD/rP5
5ucRjp6rmbwZ90VA8O8frYpV7ofXxpekvD1Q8Vrk3XwBubq01tg7a8Ugal44ApaO
X7e/X6xw6bwISe1zCCm1YKPjNKrhqcE4ujHAghVbST+sb9XiNk0TvMb1qF/dh+zx
7iCalqxZAoGBAPjNNeay5hApmoQdiyyfPwR/RzAH9eSam8Wn5pJzQz2nLFGbozmn
fO5jvI06ACumgS8LZiIGmBlbPrKQtL91Z1ftwKgBGCgqI9BpskHDP04Z/QNDlRNA
gz3qtANTmKl69RvBv82QyLzsWwcLJhVxgMTsNPnd4Z7iB9soB2mG0iFNAoGBANYK
TzDvwM6oCmtRn38zgrX+6jc2ptCAuQYeL7pn51TbljcP0XkJ8LkFaBK3lzG1NUhL
DgOcEbFEtZpwpYgDYlbVwyt3m3QUQDqm93J86pf1J1jWF81PYgUJaS7/8lBzDUiK
+PZ4XV6zYBFxUCy2yh5rxsyhBoxLV0oRD+wbGkZLAoGBAM0izYVYDY5X7xltDnoN
FrVLh9NXTOteen7+j4JCXLdxpX3n2C3KJZycSTMcFlXnI+449M2rKC8H52rtGsod
L8b0tXsP4+4ByKOm8h18sS5hCRZu23QTJeKgKCnx/BYI1h07ozwHWytBqU/mZlEZ
03UJ2CgIRGVusdGFcI8WZRylAoGASMxE1u4Uc7UvpgSi7M6GPIQxAQpzfiLpyyzl
Ks9AHNp6osucgUBiQWuXVBZhNCTftHDimVOxqMsnwRljE3mjLsmRke0iUD67Abfc
HXJjD7/v3AUlH01Kl0/2GGgw8C/RasTpnFqf1x/HIueZTzv0Tph1iw+RfJH7ZFOd
SL6HFzUCgYBpod9mhdljh4VsysZqeFfbliESb+ue7PVZb/+X9lJ7DATIq4/farhi
9YkknRAqJmKEcsomn5b35Kj0QBwiDdEE7tISdkj36jgoaz6pyyuj9ys1BlCN0fBH
2QJGFpJ3TBKqIo2iGmPHLXZVFajPF/KNVDVNlc9EUIraVgmWwDuZ7g==
-----END RSA PRIVATE KEY-----
`,
		},
		{
			name: "EC Key",
			actualPEM: `
-----BEGIN CERTIFICATE-----
MIIBeTCCAR4CCQCTj/tsh3SrEzAKBggqhkjOPQQDAjBEMQswCQYDVQQGEwJVUzEL
MAkGA1UECAwCV0ExEDAOBgNVBAcMB1JlZG1vbmQxFjAUBgNVBAMMDWRlbW8udGVz
dC5jb20wHhcNMjAxMTI0MTgzOTU1WhcNMjExMTI0MTgzOTU1WjBEMQswCQYDVQQG
EwJVUzELMAkGA1UECAwCV0ExEDAOBgNVBAcMB1JlZG1vbmQxFjAUBgNVBAMMDWRl
bW8udGVzdC5jb20wWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAQ75g7UgxCQYmWx
fn2jf6qlqaEfE45UpRsXybr1dtijtGkjE+v8I7A/GtSxfJe3LsREynlA3LGMxZL7
TD3cWsAjMAoGCCqGSM49BAMCA0kAMEYCIQDqhYQtz8uGibcOV1GCCj9emuvQqW81
DIOhxyf+tmC65gIhALNDklWc0uxg7yJQD/n1JJkkSoNdDzw9dwNGuVMHwJOY
-----END CERTIFICATE-----
-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgHv1nWow0ijr1+B4S
Vs6otqpmkzv2VRSjSPuH2zBRqQShRANCAAQ75g7UgxCQYmWxfn2jf6qlqaEfE45U
pRsXybr1dtijtGkjE+v8I7A/GtSxfJe3LsREynlA3LGMxZL7TD3cWsAj
-----END PRIVATE KEY-----
`,
			expectedKey: `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIB79Z1qMNIo69fgeElbOqLaqZpM79lUUo0j7h9swUakEoAoGCCqGSM49
AwEHoUQDQgAEO+YO1IMQkGJlsX59o3+qpamhHxOOVKUbF8m69XbYo7RpIxPr/COw
PxrUsXyXty7ERMp5QNyxjMWS+0w93FrAIw==
-----END EC PRIVATE KEY-----
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			privateKey, err := getPrivateKey([]byte(test.actualPEM))
			fmt.Println(string(privateKey))
			assert.Equal(t, test.expectedErr, err != nil)
			assert.Equal(t, test.expectedKey, string(privateKey))
		})
	}
}
