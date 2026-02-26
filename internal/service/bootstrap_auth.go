package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"hermit/internal/storage"
	"hermit/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func (s *Service) BootstrapDefaults(ctx context.Context, ownerSubject, adminPassword string) error {
	hosted, err := s.store.EnsureRepository(ctx, s.defaults.HostedRepo, store.RepoTypeHosted, nil)
	if err != nil {
		return err
	}
	group, err := s.store.EnsureRepository(ctx, s.defaults.GroupRepo, store.RepoTypeGroup, nil)
	if err != nil {
		return err
	}

	if err := s.store.UpsertRepoMember(ctx, hosted.ID, ownerSubject, store.RoleAdmin); err != nil {
		return err
	}
	if err := s.store.UpsertRepoMember(ctx, group.ID, ownerSubject, store.RoleAdmin); err != nil {
		return err
	}
	if err := s.store.UpsertRepoMember(ctx, group.ID, "*", store.RoleRead); err != nil {
		return err
	}
	if err := s.store.AddGroupMember(ctx, group.ID, hosted.ID, 10); err != nil {
		return err
	}

	if adminPassword != "" {
		if err := s.ensureAdminUser(ctx, ownerSubject, adminPassword); err != nil {
			return fmt.Errorf("bootstrap admin user: %w", err)
		}
	}

	if len(s.defaults.ProxyUpstreams) == 0 {
		return nil
	}

	for i, rawUpstream := range s.defaults.ProxyUpstreams {
		upstream := strings.TrimSpace(rawUpstream)
		if upstream == "" {
			continue
		}

		repoName := strings.TrimSpace(s.defaults.ProxyRepo)
		if repoName == "" {
			repoName = "proxy"
		}
		if i > 0 {
			repoName = fmt.Sprintf("%s-%d", repoName, i+1)
		}

		proxy, err := s.store.EnsureRepository(ctx, repoName, store.RepoTypeProxy, &upstream)
		if err != nil {
			return err
		}
		if err := s.store.UpsertRepoMember(ctx, proxy.ID, ownerSubject, store.RoleAdmin); err != nil {
			return err
		}
		if err := s.store.UpsertRepoMember(ctx, proxy.ID, "*", store.RoleRead); err != nil {
			return err
		}
		if err := s.store.AddGroupMember(ctx, group.ID, proxy.ID, 20+i); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureAdminUser(ctx context.Context, username, password string) error {
	_, err := s.store.GetUserByUsername(ctx, username)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = s.RegisterUser(ctx, username, password, "Administrator", "", true)
	return err
}

func (s *Service) BlobStore() storage.BlobStorage {
	return s.blobs
}

func (s *Service) GetReadRepository(ctx context.Context) (store.Repository, error) {
	return s.GetRepository(ctx, s.defaults.GroupRepo)
}

func (s *Service) GetPublishRepository(ctx context.Context) (store.Repository, error) {
	return s.GetRepository(ctx, s.defaults.HostedRepo)
}

func (s *Service) GetRepository(ctx context.Context, repoName string) (store.Repository, error) {
	repo, err := s.store.GetRepositoryByName(ctx, repoName)
	if err != nil {
		if store.IsNotFound(err) {
			return store.Repository{}, ErrNotFound
		}
		return store.Repository{}, err
	}
	if !repo.Enabled {
		return store.Repository{}, ErrNotFound
	}
	return repo, nil
}

func (s *Service) HasRepoPermission(ctx context.Context, repo store.Repository, subject string, requiredRole string, isAdmin bool) (bool, error) {
	if isAdmin {
		return true, nil
	}
	role, err := s.store.GetRepoRole(ctx, repo.ID, subject)
	if err != nil {
		if store.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return roleAllows(role, requiredRole), nil
}

// CreatePersonalToken creates a new personal access token for the given subject.
func (s *Service) CreatePersonalToken(ctx context.Context, subject, name string, isAdmin bool) (string, store.APIToken, error) {
	if strings.TrimSpace(subject) == "" {
		return "", store.APIToken{}, fmt.Errorf("%w: subject required", ErrInvalidInput)
	}
	if strings.TrimSpace(name) == "" {
		return "", store.APIToken{}, fmt.Errorf("%w: token name required", ErrInvalidInput)
	}

	rawToken, err := generateToken(32)
	if err != nil {
		return "", store.APIToken{}, err
	}
	sum := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(sum[:])

	id, err := s.store.CreatePersonalToken(ctx, subject, strings.TrimSpace(name), hash, isAdmin)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return "", store.APIToken{}, ErrConflict
		}
		return "", store.APIToken{}, err
	}
	tok := store.APIToken{
		ID:        id,
		Subject:   subject,
		Name:      strings.TrimSpace(name),
		TokenType: store.TokenTypePersonal,
		IsAdmin:   isAdmin,
	}
	return rawToken, tok, nil
}

// IssueSessionToken creates or replaces the session token for a subject.
// Used after successful LDAP authentication.
func (s *Service) IssueSessionToken(ctx context.Context, subject string, isAdmin bool) (string, error) {
	if strings.TrimSpace(subject) == "" {
		return "", fmt.Errorf("%w: subject required", ErrInvalidInput)
	}

	rawToken, err := generateToken(32)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(sum[:])

	if err := s.store.UpsertSessionToken(ctx, subject, hash, isAdmin); err != nil {
		return "", err
	}
	return rawToken, nil
}

// ListMyTokens returns all personal access tokens for the given subject.
func (s *Service) ListMyTokens(ctx context.Context, subject string) ([]store.APIToken, error) {
	return s.store.ListTokensBySubject(ctx, subject)
}

// RevokeToken deletes a token. Regular users can only revoke their own tokens.
func (s *Service) RevokeToken(ctx context.Context, tokenID string, subject string, isAdmin bool) error {
	id, err := uuid.Parse(tokenID)
	if err != nil {
		return fmt.Errorf("%w: invalid token ID", ErrInvalidInput)
	}
	return s.store.RevokeToken(ctx, id, subject, isAdmin)
}

// ---- Local User Management ----

// RegisterUser creates a local user account with a bcrypt-hashed password.
func (s *Service) RegisterUser(ctx context.Context, username, password, displayName, email string, isAdmin bool) (store.User, error) {
	if strings.TrimSpace(username) == "" {
		return store.User{}, fmt.Errorf("%w: username required", ErrInvalidInput)
	}
	if len(password) < 6 {
		return store.User{}, fmt.Errorf("%w: password must be at least 6 characters", ErrInvalidInput)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return store.User{}, fmt.Errorf("hash password: %w", err)
	}
	u, err := s.store.CreateUser(ctx, strings.TrimSpace(username), string(hash), strings.TrimSpace(displayName), strings.TrimSpace(email), isAdmin)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return store.User{}, fmt.Errorf("%w: username already exists", ErrConflict)
		}
		return store.User{}, err
	}
	return u, nil
}

// LocalLogin authenticates a local user by username and password, then issues a session token.
func (s *Service) LocalLogin(ctx context.Context, username, password string) (string, store.User, error) {
	if strings.TrimSpace(username) == "" || password == "" {
		return "", store.User{}, fmt.Errorf("%w: username and password required", ErrInvalidInput)
	}
	u, err := s.store.GetUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", store.User{}, fmt.Errorf("invalid username or password")
		}
		return "", store.User{}, err
	}
	if u.Disabled {
		return "", store.User{}, fmt.Errorf("account disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", store.User{}, fmt.Errorf("invalid username or password")
	}
	token, err := s.IssueSessionToken(ctx, u.Username, u.IsAdmin)
	if err != nil {
		return "", store.User{}, err
	}
	return token, u, nil
}

// ListUsers returns all local user accounts (admin only).
func (s *Service) ListUsers(ctx context.Context) ([]store.User, error) {
	return s.store.ListUsers(ctx)
}

// UpdateUser updates a local user's profile and role (admin only).
func (s *Service) UpdateUser(ctx context.Context, userID string, displayName, email string, isAdmin, disabled bool) (store.User, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return store.User{}, fmt.Errorf("%w: invalid user ID", ErrInvalidInput)
	}
	u, err := s.store.UpdateUser(ctx, id, strings.TrimSpace(displayName), strings.TrimSpace(email), isAdmin, disabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.User{}, ErrNotFound
		}
		return store.User{}, err
	}
	return u, nil
}

// ResetUserPassword resets a user's password (admin only).
func (s *Service) ResetUserPassword(ctx context.Context, userID, newPassword string) error {
	if len(newPassword) < 6 {
		return fmt.Errorf("%w: password must be at least 6 characters", ErrInvalidInput)
	}
	id, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("%w: invalid user ID", ErrInvalidInput)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.store.UpdateUserPassword(ctx, id, string(hash)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// DeleteUser removes a local user account (admin only).
func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	id, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("%w: invalid user ID", ErrInvalidInput)
	}
	return s.store.DeleteUser(ctx, id)
}
