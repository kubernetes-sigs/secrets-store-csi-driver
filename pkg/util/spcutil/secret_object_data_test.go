/*
Copyright 2021 The Kubernetes Authors.

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

package spcutil

import (
	"reflect"
	"sort"
	"testing"

	corev1 "k8s.io/api/core/v1"
	secretsstorev1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

const (
	cert = `
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

	sshKey = `
-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAACmFlczI1Ni1jdHIAAAAGYmNyeXB0AAAAGAAAABBa+0SeK0
uw8TTDVmQuIbT+AAAAEAAAAAEAAAGXAAAAB3NzaC1yc2EAAAADAQABAAABgQCrAMIhG+1R
tG5kgrmieNDE+hjblN64keV0EnsaSpCKCBFFnWQGCOaNZaoIJVX088IFCzTkz0QeLHl7qO
cSeBDxdm/GTvphEyh5JTroV+ipDq6+6tJ2YNLksP+NmMW5/A3/byFK7cSglwupJgBdMX7l
oAFZLMCOBiLC4CTHmA74xRHkwHgdNZUiiUzw0eC1fUOoQWKT6W+rsCV/Tk6hh0X7x+qLO8
GnT8S2GzapMY0HoJTK5yV4m/0KV/Ky90OfTDZnAAHEYjQYvQ2YY0UEcU6KZ71msZZ2XCPY
p1mVpMXJ9b2KCOOMKm8ursla79plYmH6cUaAsonT182rD0lHIZPHFxcRZpZGs5Zp/iyZ41
MK4vlKwEMRdn9KjByTaS21fOHDO3De9LUwg4Q0DmsUrp8M0lG8BX4fJl51v5TCYh2+X2ds
RIsJenJ24Dmuv5tQrWmr6JMxu3xccUHN/NBMYGESEXjMWKAtSfssARbVI8QRJn21N/838C
JV57cv5l7dI7EAAAWQB3gJeXmZjEBVrHOM77c4bXqlhm55jWwMj79N/jc6lGGmESArjbfY
g85wYR0LEsiXETTnkwpCmCRAuUKj5+vAGyDeWM76KGp4igl8RExF8bv74RFUyN3A7O2EbM
HQ2xYoL4W1KX67AT5KJD3g+4nkV74DWhdlazA2DspL7jSz6yDfDXeYxsxIozvkEhoac3T7
u1UbfvDZw7MnW95gc3ybm74DH1nso2TQzvHEN9Vq/hXa50e7lJtXJ1nVfIBxda0VNqCAdo
CnTG5Q5UKc2l4dfhqScCwbBWjO2M2YPYMBOUSayigQ6S1sJCYL3TpGUl290Q1cWFWQ+ORl
EfeGe9TKxnY64smQH2LftuO5WDmwvf1Ske5BATtZX6zOUpwjgAQ1Bpj0wpht54O8Rbo8ub
kmh+uaE6k0iUEN2van3Mkq9ok/62J+QeFbK7V+Ag2+l4MI7f5cbfcwyMRyLaLIPvTfQWPQ
3RtQdbQHdcGvfmu6aPkKhN5/uoyBbmWOvIsnlYiZp4oY15CFtIYIyhyaHTm1/2rZnuyC6q
Q6dlWxkZEF6p7Uh7dB+9x/GR+MfCLiFNuYms//r1ZEtYp2ctIgL4W2z09a2xN9jJ4Ij3Kn
amJ/NUen5EiTEdNwZlAtpwfh6/x7eQnbq3bMmJcFdrt27lHQg6k2NOKsDxEoAEn5q3fySH
XRT848yHyDXhCht0AooeFk/ToyzjSWlRTPjgJZ+ryckwqOB/D4UvQH5r5SWhQtTfzGUoO7
9Ozmj4CiCMtlrdcDiQTbgyR1BPReyoR5Xq+m+S2i0kFzGedTalFxhrhHCoSiuy4YcKCcVd
+xQuuJ7HrEkY78EMoII/Al8TKTyD6FWQGWoqWEVcwBXV4J3KUkk1N3oEaqQwj/TXpGlzUf
17QvEjDepHlCbI/5KBUCl8gzc3ByisRWx6a67VbOOih61304VUHapcAJmduiOBnnSFj2kb
3BwANN5uk+lzbeyWF83VthZXr2WcWDqE/g1Ox87TkBHE8JePlxc3/bPkeD8feC11YmNJzX
91kU2XPOxplD11fCCpIh2tbjRUR1VVpoVuf75FwsUo3ecqSSJ8MbpQduGKCllpBuvo2yLQ
A9Bn836/bEvzbpQhTPrPCC7/Nk9QL+PtgfROcW2uOHiMrNZrAyghboV4KPPtmB9RWCxqoY
viWQMn4wIShrg1qluZBpKrNUr7NedjU7MjY6FMK9nernY4YllQB77/MkJNm74Gu8meRGSn
cGQA3P/xcQXR3QQ6spXTfARUSoDF0eNALhoEKwhFcPGNc8nI1g2Ach/Px567aKuOuThOPT
2Ue5/pe4YzMHKOiStLMgbYD31k9KKjvqNbLV1DJFHb7fPymY3GhfrtxemyX1jbZFxUfiKr
WrUuOa/Vzi1fkrTdC62TGHKlWmQD8dUuuuioTbWyAAmPODcsjurXxQVzTJqO8bwrQGKjEc
E/UPV31EMHWlYOQ35lKYMkxyWXeHA9UhGaC3ns+TMzeRRgbX7LRjhwPo4y1gUo4wEjrqyd
PpLSGRRtq8JVFonxnf5xttHJegXKhVb2VkEquZ2jL3GcQyCh6oaKo0yNIdOuQhI1aA3oxZ
f+Eag3u3IzfZYCBnXsRfZ3j9+L+VmTz4WJx6iRHSQRPrSBzVi3U0I5N+ohxzyhORWr2tpp
jcYZ59Pzboy7FXp5s4JK0TOP+XCK9h6v5T7V5fW6XS5fABiVl94zVc1THnJnmHhBIc+7Dm
hlUYwT+6TWxDOJo8sqp6+NLLTx2UxOqKUWWCUCSvap2ptOvIZRm2Cq5kW3W2FFo7tSftWt
d9H7wnDSuC4Zv1ze8Wo2TbXv222TIysd8rK4j+0dTDibvZ1WVbFs00cEBXKIZxskrSlAIb
C9I7vbQHcdYGWqof7MBPsMcaJ+0=
-----END OPENSSH PRIVATE KEY-----
	
`

	dockerconfigjson = "{\"auths\":{\"https://index.docker.io/v1/\":{\"username\":\"user1\",\"password\":\"password1\",\"email\":\"account1@email.com\",\"auth\":\"dXNlcjE6cGFzc3dvcmQx\"}}}"
)

func TestBuildSecretObjectData(t *testing.T) {
	files := map[string]string{
		"username":               "test user",
		"password":               "test password",
		"nested/username":        "test nested user",
		"nested/double/username": "double nested user",
	}

	secretObj := &secretsstorev1.SecretObject{
		SecretName: "test-secret",
		Type:       "Opaque",
		SyncAll:    true,
	}

	BuildSecretObjectData(files, secretObj)

	expected := &secretsstorev1.SecretObject{
		SecretName: "test-secret",
		Type:       "Opaque",
		SyncAll:    true,
		Data: []*secretsstorev1.SecretObjectData{
			{
				ObjectName: "username",
				Key:        "username",
			},
			{
				ObjectName: "password",
				Key:        "password",
			},
			{
				ObjectName: "nested/username",
				Key:        "nested-username",
			},
			{
				ObjectName: "nested/double/username",
				Key:        "nested-double-username",
			},
		},
	}

	if ok := assertSecretsObjectsEqual(expected, secretObj); !ok {
		t.Fatal("secret objects did not match")
	}
}

func TestBuildSecretObjects(t *testing.T) {
	tests := []struct {
		files      map[string]string
		secretType corev1.SecretType
		expected   []*secretsstorev1.SecretObject
	}{
		{
			files: map[string]string{
				"username":               "test user",
				"password":               "test password",
				"nested/username":        "test nested user",
				"nested/double/username": "double nested user",
			},
			secretType: corev1.SecretTypeOpaque,
			expected: []*secretsstorev1.SecretObject{
				{
					SecretName: "username",
					Type:       string(corev1.SecretTypeOpaque),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "username",
							Key:        "username",
						},
					},
				},
				{
					SecretName: "password",
					Type:       string(corev1.SecretTypeOpaque),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "password",
							Key:        "password",
						},
					},
				},
				{
					SecretName: "nested-username",
					Type:       string(corev1.SecretTypeOpaque),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "nested/username",
							Key:        "username",
						},
					},
				},
				{
					SecretName: "nested-double-username",
					Type:       string(corev1.SecretTypeOpaque),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "nested/double/username",
							Key:        "username",
						},
					},
				},
			},
		}, {
			files: map[string]string{
				"cert": cert,
			},
			secretType: corev1.SecretTypeTLS,
			expected: []*secretsstorev1.SecretObject{
				{
					SecretName: "cert",
					Type:       string(corev1.SecretTypeTLS),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "cert",
							Key:        "tls.key",
						},
						{
							ObjectName: "cert",
							Key:        "tls.crt",
						},
					},
				},
			},
		}, {
			files: map[string]string{
				"basic/basic1": "my-username1,my-password1",
				"basic/basic2": "my-username2,my-password2",
				"basic/basic3": "my-username3,my-password3",
			},
			secretType: corev1.SecretTypeBasicAuth,
			expected: []*secretsstorev1.SecretObject{
				{
					SecretName: "basic-basic1",
					Type:       string(corev1.SecretTypeBasicAuth),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "basic/basic1",
							Key:        "basic1",
						},
					},
				},
				{
					SecretName: "basic-basic2",
					Type:       string(corev1.SecretTypeBasicAuth),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "basic/basic2",
							Key:        "basic2",
						},
					},
				},
				{
					SecretName: "basic-basic3",
					Type:       string(corev1.SecretTypeBasicAuth),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "basic/basic3",
							Key:        "basic3",
						},
					},
				},
			},
		}, {
			files: map[string]string{
				"ssh": sshKey,
			},
			secretType: corev1.SecretTypeSSHAuth,
			expected: []*secretsstorev1.SecretObject{
				{
					SecretName: "ssh",
					Type:       string(corev1.SecretTypeSSHAuth),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "ssh",
							Key:        "ssh-privatekey",
						},
					},
				},
			},
		},
		{
			files: map[string]string{
				"dev/docker": dockerconfigjson,
			},
			secretType: corev1.SecretTypeDockerConfigJson,
			expected: []*secretsstorev1.SecretObject{
				{
					SecretName: "dev-docker",
					Type:       string(corev1.SecretTypeDockerConfigJson),
					Data: []*secretsstorev1.SecretObjectData{
						{
							ObjectName: "dev/docker",
							Key:        ".dockerconfigjson",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		actualSecretObjects := BuildSecretObjects(test.files, test.secretType)
		if ok := assertSecretObjectSlicesEqual(test.expected, actualSecretObjects); !ok {
			t.Fatal("secret object slices did not match")
		}
	}

}

func assertSecretsObjectsEqual(expected, actual *secretsstorev1.SecretObject) bool {
	if expected.SecretName != actual.SecretName {
		return false
	}

	if expected.Type != actual.Type {
		return false
	}

	sort.Slice(expected.Data, func(i, j int) bool {
		return expected.Data[i].ObjectName < expected.Data[j].ObjectName
	})

	sort.Slice(actual.Data, func(i, j int) bool {
		return actual.Data[i].ObjectName < actual.Data[j].ObjectName
	})

	return reflect.DeepEqual(expected.Data, actual.Data)
}

func assertSecretObjectSlicesEqual(expected, actual []*secretsstorev1.SecretObject) bool {
	if len(expected) != len(actual) {
		return false
	}

	sort.Slice(expected, func(i, j int) bool {
		return expected[i].SecretName < expected[j].SecretName
	})

	sort.Slice(actual, func(i, j int) bool {
		return actual[i].SecretName < actual[j].SecretName
	})

	for i := 0; i < len(expected); i++ {
		if ok := assertSecretsObjectsEqual(expected[i], actual[i]); !ok {
			return false
		}
	}

	return true
}
