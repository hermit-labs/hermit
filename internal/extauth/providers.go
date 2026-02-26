package extauth

// Provider describes an available authentication method.
type Provider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "token", "ldap", "standard"
}

// IdentityResult is returned after successful external authentication.
type IdentityResult struct {
	Subject     string
	DisplayName string
	Email       string
	IsAdmin     bool
}
