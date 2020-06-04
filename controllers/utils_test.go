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

package controllers

import (
	"testing"

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
		actualPEM, err := getCertPart([]byte(tc.data), tc.part)
		assert.Equal(t, tc.expectedErr, err != nil)
		assert.Equal(t, tc.expectedPEM, actualPEM)
	}
}
