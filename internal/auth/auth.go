package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type Claims struct {
	Subject string
	IsAdmin bool
}

// Actor represents the identity performing a request.
// Anonymous is true when no valid token was provided.
type Actor struct {
	Subject   string
	IsAdmin   bool
	Anonymous bool
}

// AnonymousActor returns an Actor for unauthenticated requests.
func AnonymousActor() Actor {
	return Actor{Anonymous: true}
}

// ActorFromClaims converts authenticated Claims into an Actor.
func ActorFromClaims(c Claims) Actor {
	return Actor{Subject: c.Subject, IsAdmin: c.IsAdmin}
}

const claimsContextKey = "auth_claims"

type Authenticator struct {
	db         *pgxpool.Pool
	adminToken string
}

func NewAuthenticator(db *pgxpool.Pool, adminToken string) *Authenticator {
	return &Authenticator{
		db:         db,
		adminToken: adminToken,
	}
}

// Middleware requires a valid token; rejects unauthenticated requests.
func (a *Authenticator) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := extractToken(c.Request())
		if token == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "missing API token")
		}

		claims, err := a.Authenticate(c.Request().Context(), token)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid API token")
		}
		c.Set(claimsContextKey, claims)

		return next(c)
	}
}

// OptionalMiddleware sets claims if a valid token is present but does not
// reject anonymous requests. Use on public endpoints that benefit from
// knowing the caller's identity (e.g. repo-level filtering).
func (a *Authenticator) OptionalMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := extractToken(c.Request())
		if token != "" {
			claims, err := a.Authenticate(c.Request().Context(), token)
			if err == nil {
				c.Set(claimsContextKey, claims)
			}
		}
		return next(c)
	}
}

func (a *Authenticator) Authenticate(ctx context.Context, token string) (Claims, error) {
	if token == a.adminToken {
		return Claims{Subject: "admin", IsAdmin: true}, nil
	}

	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	var tokenID string
	var subject string
	var isAdmin bool
	var disabled bool
	err := a.db.QueryRow(ctx, `
		SELECT id, subject, is_admin, disabled
		FROM api_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(&tokenID, &subject, &isAdmin, &disabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Claims{}, err
		}
		return Claims{}, err
	}
	if disabled {
		return Claims{}, errors.New("token disabled")
	}

	// Async update last_used_at
	go func() {
		_, _ = a.db.Exec(context.Background(),
			`UPDATE api_tokens SET last_used_at = now() WHERE id = $1`, tokenID)
	}()

	return Claims{Subject: subject, IsAdmin: isAdmin}, nil
}

func GetClaims(c echo.Context) (Claims, bool) {
	raw := c.Get(claimsContextKey)
	if raw == nil {
		return Claims{}, false
	}
	claims, ok := raw.(Claims)
	return claims, ok
}

// GetActor returns the Actor for the current request. If the user is
// authenticated, the Actor contains their identity; otherwise it is anonymous.
func GetActor(c echo.Context) Actor {
	claims, ok := GetClaims(c)
	if !ok {
		return AnonymousActor()
	}
	return ActorFromClaims(claims)
}

func extractToken(r *http.Request) string {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	return strings.TrimSpace(r.Header.Get("X-API-Token"))
}
