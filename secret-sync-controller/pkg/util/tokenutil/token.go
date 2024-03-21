package tokenutil

// TokenRequest contains parameters of a service account token.
type TokenRequest struct {
	// Audience is the intended audience of the token in "TokenRequestSpec".
	// It will default to the audiences of kube apiserver.
	//
	Audience string `json:"audience"`

	// ExpirationSeconds is the duration of validity of the token in "TokenRequestSpec".
	// It has the same default value of "ExpirationSeconds" in "TokenRequestSpec".
	//
	// +optional
	ExpirationSeconds *int64 `json:"expirationSeconds,omitempty"`
}
