package server

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

func TestMount(t *testing.T) {
	server := &SimpleCSIProviderServer{}

	cases := []struct {
		name    string
		input   *v1alpha1.MountRequest
		want    *v1alpha1.MountResponse
		wantErr error
	}{
		{
			"Parse static secrets",
			&v1alpha1.MountRequest{
				Attributes: func() string {
					attributes := map[string]string{
						// not a real token, just for testing
						"csi.storage.k8s.io/serviceAccount.tokens": `{"https://kubernetes.default.svc":{"token":"eyJhbGciOiJSUzI1NiIsImtpZCI6IjEyMyJ9.eyJhdWQiOlsiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjIl0sImV4cCI6MTYxMTk1OTM5NiwiaWF0IjoxNjExOTU4Nzk2LCJpc3MiOiJodHRwczovL2t1YmVybmV0ZXMuZGVmYXVsdC5zdmMiLCJrdWJlcm5ldGVzLmlvIjp7Im5hbWVzcGFjZSI6ImRlZmF1bHQiLCJzZXJ2aWNlYWNjb3VudCI6eyJuYW1lIjoiZGVmYXVsdCIsInVpZCI6IjA5MWUyNTU3LWJkODYtNDhhMC1iZmNmLWI1YTI4ZjRjODAyNCJ9fSwibmJmIjoxNjExOTU4Nzk2LCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6ZGVmYXVsdDpkZWZhdWx0In0.YNU2Z_gEE84DGCt8lh9GuE8gmoof-Pk_7emp3fsyj9pq16DRiDaLtOdprH-njpOYqvtT5Uf_QspFc_RwD_pdq9UJWCeLxFkRTsYR5WSjhMFcl767c4Cwp_oZPYhaHd1x7aU1emH-9oarrM__tr1hSmGoAc2I0gUSkAYFueaTUSy5e5d9QKDfjVljDRc7Yrp6qAAfd1OuDdk1XYIjrqTHk1T1oqGGlcd3lRM_dKSsW5I_YqgKMrjwNt8yOKcdKBrgQhgC42GZbFDRVJDJHs_Hq32xo-2s3PJ8UZ_alN4wv8EbuwB987_FHBTc_XAULHPvp0mCv2C5h0V2A7gzccv30A","expirationTimestamp":"2021-01-29T22:29:56Z"}}`,
						"secrets": `- key: "username"
  value: "local-user"
- key: "password"
  value: "1234"`,
					}
					data, _ := json.Marshal(attributes)
					return string(data)
				}(),
				Secrets:    "{}",
				Permission: "640",
				TargetPath: "/",
			},
			&v1alpha1.MountResponse{
				ObjectVersion: []*v1alpha1.ObjectVersion{
					{
						Id:      "secret/kubernetes.default.svc",
						Version: "v1",
					},
					{
						Id:      "secret/username",
						Version: "v1",
					},
					{
						Id:      "secret/password",
						Version: "v1",
					},
					{
						Id:      "secret/foo",
						Version: "v1",
					},
					{
						Id:      "secret/fookey",
						Version: "v1",
					},
				},
				Files: []*v1alpha1.File{
					{
						Path:     "kubernetes.default.svc",
						Contents: []byte(`eyJhbGciOiJSUzI1NiIsImtpZCI6IjEyMyJ9.eyJhdWQiOlsiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjIl0sImV4cCI6MTYxMTk1OTM5NiwiaWF0IjoxNjExOTU4Nzk2LCJpc3MiOiJodHRwczovL2t1YmVybmV0ZXMuZGVmYXVsdC5zdmMiLCJrdWJlcm5ldGVzLmlvIjp7Im5hbWVzcGFjZSI6ImRlZmF1bHQiLCJzZXJ2aWNlYWNjb3VudCI6eyJuYW1lIjoiZGVmYXVsdCIsInVpZCI6IjA5MWUyNTU3LWJkODYtNDhhMC1iZmNmLWI1YTI4ZjRjODAyNCJ9fSwibmJmIjoxNjExOTU4Nzk2LCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6ZGVmYXVsdDpkZWZhdWx0In0.YNU2Z_gEE84DGCt8lh9GuE8gmoof-Pk_7emp3fsyj9pq16DRiDaLtOdprH-njpOYqvtT5Uf_QspFc_RwD_pdq9UJWCeLxFkRTsYR5WSjhMFcl767c4Cwp_oZPYhaHd1x7aU1emH-9oarrM__tr1hSmGoAc2I0gUSkAYFueaTUSy5e5d9QKDfjVljDRc7Yrp6qAAfd1OuDdk1XYIjrqTHk1T1oqGGlcd3lRM_dKSsW5I_YqgKMrjwNt8yOKcdKBrgQhgC42GZbFDRVJDJHs_Hq32xo-2s3PJ8UZ_alN4wv8EbuwB987_FHBTc_XAULHPvp0mCv2C5h0V2A7gzccv30A`),
					},
					{
						Path:     "username",
						Contents: []byte("local-user"),
					},
					{
						Path:     "password",
						Contents: []byte("1234"),
					},
					{
						Path:     "foo",
						Contents: []byte("bar"),
					},
					{
						Path:     "fookey",
						Contents: []byte("barkey"),
					},
				},
			},
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := server.Mount(context.Background(), tc.input)
			if tc.wantErr != nil {
				if err == nil {
					t.Errorf("Did not receive expected error: %v", tc.wantErr)
					return
				}
				if tc.wantErr.Error() != err.Error() {
					t.Errorf("Received unexpected error: wanted %v, got %v", tc.wantErr, err)
					return
				}
			} else {
				if err != nil {
					t.Errorf("Received unexpected error: got %v", err)
					return
				}
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tc.want)
			if !reflect.DeepEqual(gotJSON, wantJSON) {
				t.Errorf("Didn't get expected results: wanted \n%s\n    got \n%s", string(wantJSON), string(gotJSON))
			}
		})
	}
}
