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

func (a *Authenticator) Authenticate(ctx context.Context, token string) (Claims, error) {
	if token == a.adminToken {
		return Claims{Subject: "admin", IsAdmin: true}, nil
	}

	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	var subject string
	var disabled bool
	err := a.db.QueryRow(ctx, `
		SELECT subject, disabled
		FROM api_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(&subject, &disabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Claims{}, err
		}
		return Claims{}, err
	}
	if disabled {
		return Claims{}, errors.New("token disabled")
	}

	return Claims{Subject: subject}, nil
}

func GetClaims(c echo.Context) (Claims, bool) {
	raw := c.Get(claimsContextKey)
	if raw == nil {
		return Claims{}, false
	}
	claims, ok := raw.(Claims)
	return claims, ok
}

func extractToken(r *http.Request) string {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	return strings.TrimSpace(r.Header.Get("X-API-Token"))
}
