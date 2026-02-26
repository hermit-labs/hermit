package handlers

import (
	"net/http"

	"hermit/internal/extauth"
	"hermit/internal/service"

	"github.com/labstack/echo/v4"
)

// AuthProviders returns the list of enabled authentication methods.
func (h *Handler) AuthProviders(c echo.Context) error {
	ctx := c.Request().Context()
	providers := []extauth.Provider{
		{ID: "standard", Name: "Standard", Type: "standard"},
	}
	if h.svc.IsAuthProviderEnabled(ctx, service.ProviderTypeLDAP) {
		providers = append(providers, extauth.Provider{ID: "ldap", Name: "LDAP", Type: "ldap"})
	}
	return c.JSON(http.StatusOK, map[string]any{"providers": providers})
}

// LocalLogin authenticates against the local users table.
func (h *Handler) LocalLogin(c echo.Context) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	token, user, err := h.svc.LocalLogin(c.Request().Context(), body.Username, body.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"token":        token,
		"subject":      user.Username,
		"display_name": user.DisplayName,
		"email":        user.Email,
		"is_admin":     user.IsAdmin,
	})
}

// LDAPLogin authenticates via LDAP and returns a session token.
func (h *Handler) LDAPLogin(c echo.Context) error {
	ctx := c.Request().Context()
	ldapAuth, err := h.svc.GetLDAPAuthenticator(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load LDAP config")
	}
	if ldapAuth == nil {
		return echo.NewHTTPError(http.StatusNotFound, "LDAP not enabled")
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	identity, err := ldapAuth.Authenticate(body.Username, body.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	token, err := h.svc.IssueSessionToken(ctx, identity.Subject, identity.IsAdmin)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to issue token")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"token":        token,
		"subject":      identity.Subject,
		"display_name": identity.DisplayName,
		"email":        identity.Email,
		"is_admin":     identity.IsAdmin,
	})
}
