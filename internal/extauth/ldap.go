package extauth

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// LDAPConfig holds LDAP connection and search parameters.
type LDAPConfig struct {
	URL          string   `json:"url"`
	BaseDN       string   `json:"base_dn"`
	BindDN       string   `json:"bind_dn"`
	BindPassword string   `json:"bind_password"`
	UserFilter   string   `json:"user_filter"`
	UserAttr     string   `json:"user_attr"`
	DisplayAttr  string   `json:"display_attr"`
	StartTLS     bool     `json:"start_tls"`
	SkipVerify   bool     `json:"skip_verify"`
	AdminGroups  []string `json:"admin_groups"`
}

// LDAPAuthenticator performs bind-based LDAP authentication.
type LDAPAuthenticator struct {
	cfg LDAPConfig
}

func NewLDAPAuthenticator(cfg LDAPConfig) *LDAPAuthenticator {
	return &LDAPAuthenticator{cfg: cfg}
}

// Authenticate verifies the username/password against LDAP and returns identity info.
func (la *LDAPAuthenticator) Authenticate(username, password string) (*IdentityResult, error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return nil, fmt.Errorf("username and password required")
	}

	conn, err := la.connect()
	if err != nil {
		return nil, fmt.Errorf("ldap connect: %w", err)
	}
	defer conn.Close()

	// Service account bind to search for the user
	if la.cfg.BindDN != "" {
		if err := conn.Bind(la.cfg.BindDN, la.cfg.BindPassword); err != nil {
			return nil, fmt.Errorf("ldap service bind: %w", err)
		}
	}

	userDN, attrs, err := la.searchUser(conn, username)
	if err != nil {
		return nil, err
	}

	// Bind as the user to verify their password
	if err := conn.Bind(userDN, password); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	subject := username
	if v, ok := attrs[la.cfg.UserAttr]; ok && v != "" {
		subject = v
	}
	displayName := subject
	if v, ok := attrs[la.cfg.DisplayAttr]; ok && v != "" {
		displayName = v
	}
	email, _ := attrs["mail"]

	return &IdentityResult{
		Subject:     subject,
		DisplayName: displayName,
		Email:       email,
	}, nil
}

func (la *LDAPAuthenticator) connect() (*ldap.Conn, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: la.cfg.SkipVerify}

	if strings.HasPrefix(la.cfg.URL, "ldaps://") {
		return ldap.DialURL(la.cfg.URL, ldap.DialWithTLSConfig(tlsCfg))
	}

	conn, err := ldap.DialURL(la.cfg.URL)
	if err != nil {
		return nil, err
	}

	if la.cfg.StartTLS {
		if err := conn.StartTLS(tlsCfg); err != nil {
			conn.Close()
			return nil, fmt.Errorf("starttls: %w", err)
		}
	}
	return conn, nil
}

func (la *LDAPAuthenticator) searchUser(conn *ldap.Conn, username string) (string, map[string]string, error) {
	filter := strings.ReplaceAll(la.cfg.UserFilter, "{{.Username}}", ldap.EscapeFilter(username))

	searchAttrs := []string{"dn", la.cfg.UserAttr, la.cfg.DisplayAttr, "mail"}

	result, err := conn.Search(ldap.NewSearchRequest(
		la.cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1,    // size limit
		10,   // time limit seconds
		false,
		filter,
		searchAttrs,
		nil,
	))
	if err != nil {
		return "", nil, fmt.Errorf("ldap search: %w", err)
	}
	if len(result.Entries) == 0 {
		return "", nil, fmt.Errorf("user not found")
	}

	entry := result.Entries[0]
	attrs := make(map[string]string, len(searchAttrs))
	for _, a := range searchAttrs {
		if a != "dn" {
			attrs[a] = entry.GetAttributeValue(a)
		}
	}
	return entry.DN, attrs, nil
}
