package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"hermit/internal/extauth"
	"hermit/internal/store"

	"github.com/jackc/pgx/v5"
)

const (
	ProviderTypeLDAP = "ldap"
)

// AuthConfigView is the API representation with sensitive fields masked.
type AuthConfigView struct {
	ProviderType string `json:"provider_type"`
	Enabled      bool   `json:"enabled"`
	Config       any    `json:"config"`
	UpdatedAt    string `json:"updated_at"`
}

// cached authenticator instances, rebuilt when config changes.
var (
	ldapMu      sync.Mutex
	cachedLDAP  *extauth.LDAPAuthenticator
	ldapVersion int64
)

func (s *Service) ListAuthConfigs(ctx context.Context) ([]AuthConfigView, error) {
	configs, err := s.store.ListAuthConfigs(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]AuthConfigView, 0, len(configs))
	for _, c := range configs {
		views = append(views, toAuthConfigView(c))
	}
	return views, nil
}

func (s *Service) GetAuthConfig(ctx context.Context, providerType string) (AuthConfigView, error) {
	ac, err := s.store.GetAuthConfig(ctx, providerType)
	if err != nil {
		return AuthConfigView{}, err
	}
	return toAuthConfigView(ac), nil
}

func (s *Service) SaveAuthConfig(ctx context.Context, providerType string, enabled bool, rawConfig json.RawMessage) error {
	switch providerType {
	case ProviderTypeLDAP:
		var cfg extauth.LDAPConfig
		if err := json.Unmarshal(rawConfig, &cfg); err != nil {
			return fmt.Errorf("%w: invalid LDAP config: %v", ErrInvalidInput, err)
		}
	default:
		return fmt.Errorf("%w: unknown provider type %q", ErrInvalidInput, providerType)
	}

	if err := s.store.UpsertAuthConfig(ctx, providerType, enabled, rawConfig); err != nil {
		return err
	}

	if providerType == ProviderTypeLDAP {
		ldapMu.Lock()
		cachedLDAP = nil
		ldapVersion++
		ldapMu.Unlock()
	}
	return nil
}

func (s *Service) DeleteAuthConfig(ctx context.Context, providerType string) error {
	if err := s.store.DeleteAuthConfig(ctx, providerType); err != nil {
		return err
	}
	if providerType == ProviderTypeLDAP {
		ldapMu.Lock()
		cachedLDAP = nil
		ldapVersion++
		ldapMu.Unlock()
	}
	return nil
}

// GetLDAPAuthenticator returns a cached LDAP authenticator built from DB config.
// Returns nil if LDAP is not configured or disabled.
func (s *Service) GetLDAPAuthenticator(ctx context.Context) (*extauth.LDAPAuthenticator, error) {
	ldapMu.Lock()
	if cachedLDAP != nil {
		defer ldapMu.Unlock()
		return cachedLDAP, nil
	}
	ldapMu.Unlock()

	ac, err := s.store.GetAuthConfig(ctx, ProviderTypeLDAP)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if !ac.Enabled {
		return nil, nil
	}

	var cfg extauth.LDAPConfig
	if err := json.Unmarshal(ac.Config, &cfg); err != nil {
		return nil, fmt.Errorf("parse LDAP config: %w", err)
	}

	auth := extauth.NewLDAPAuthenticator(cfg)
	ldapMu.Lock()
	cachedLDAP = auth
	ldapMu.Unlock()
	return auth, nil
}

// IsAuthProviderEnabled checks if a given provider type is enabled in the DB.
func (s *Service) IsAuthProviderEnabled(ctx context.Context, providerType string) bool {
	ac, err := s.store.GetAuthConfig(ctx, providerType)
	if err != nil {
		return false
	}
	return ac.Enabled
}

// SeedAuthConfigFromEnv writes LDAP config into the DB if not already present.
// Called during bootstrap to migrate env var config to DB.
func (s *Service) SeedAuthConfigFromEnv(ctx context.Context, providerType string, enabled bool, rawConfig json.RawMessage) error {
	_, err := s.store.GetAuthConfig(ctx, providerType)
	if err == nil {
		return nil // already exists, don't overwrite
	}
	if err != pgx.ErrNoRows {
		return err
	}
	return s.store.UpsertAuthConfig(ctx, providerType, enabled, rawConfig)
}

func toAuthConfigView(ac store.AuthConfig) AuthConfigView {
	var config any
	_ = json.Unmarshal(ac.Config, &config)

	if m, ok := config.(map[string]any); ok {
		for _, key := range []string{"bind_password"} {
			if v, exists := m[key]; exists {
				if s, ok := v.(string); ok && len(s) > 0 {
					m[key] = "••••••••"
				}
			}
		}
	}

	return AuthConfigView{
		ProviderType: ac.ProviderType,
		Enabled:      ac.Enabled,
		Config:       config,
		UpdatedAt:    ac.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
