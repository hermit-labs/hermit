package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"hermit/internal/auth"
	"hermit/internal/service"
	"hermit/internal/store"

	"github.com/labstack/echo/v4"
)

func (h *Handler) GetDashboardStats(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}

	stats, err := h.svc.GetDashboardStats(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, stats)
}

func (h *Handler) ListSyncSources(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}

	sources, err := h.svc.ListSyncSources(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"sources": sources})
}

func (h *Handler) AddSyncSource(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}

	var req struct {
		Name        string `json:"name"`
		UpstreamURL string `json:"upstreamUrl"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	source, err := h.svc.AddSyncSource(c.Request().Context(), req.Name, req.UpstreamURL)
	if err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, source)
}

func (h *Handler) RemoveSyncSource(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}

	id := strings.TrimSpace(c.Param("id"))
	if err := h.svc.RemoveSyncSource(c.Request().Context(), id); err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) ToggleSyncSource(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}

	id := strings.TrimSpace(c.Param("id"))
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	if err := h.svc.ToggleSyncSource(c.Request().Context(), id, req.Enabled); err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) TriggerSync(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}

	if h.syncTrigger == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "sync not configured")
	}

	started, err := h.syncTrigger.TriggerSync(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if !started {
		return c.JSON(http.StatusOK, map[string]any{
			"ok":      true,
			"message": "sync already running",
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"ok":      true,
		"message": "sync started",
	})
}

func (h *Handler) GetSyncStatus(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}

	if h.syncTrigger == nil {
		return c.JSON(http.StatusOK, map[string]any{
			"configured": false,
			"running":    false,
		})
	}

	status := h.syncTrigger.Status()
	return c.JSON(http.StatusOK, map[string]any{
		"configured": true,
		"running":    status.Running,
		"lastResult": status.LastResult,
		"lastError":  status.LastError,
	})
}

// --- Proxy Sync Config ---

func (h *Handler) GetProxySyncConfig(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}
	cfg, err := h.svc.GetProxySyncConfig(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, cfg)
}

func (h *Handler) SaveProxySyncConfig(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}
	var req service.ProxySyncConfig
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if err := h.svc.SaveProxySyncConfig(c.Request().Context(), req); err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// --- RBAC Management Handlers ---

func (h *Handler) ListAllMembers(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}

	members, err := h.svc.ListAllRepoMembers(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"members": members})
}

func (h *Handler) ListMembers(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}

	repoID := strings.TrimSpace(c.Param("id"))
	members, err := h.svc.ListRepoMembers(c.Request().Context(), repoID)
	if err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"members": members})
}

func (h *Handler) AssignMember(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}

	repoID := strings.TrimSpace(c.Param("id"))
	var req struct {
		Subject string `json:"subject"`
		Role    string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	if err := h.svc.AssignRepoRole(c.Request().Context(), repoID, req.Subject, req.Role); err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) RemoveMember(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}

	repoID := strings.TrimSpace(c.Param("id"))
	subject := strings.TrimSpace(c.Param("subject"))

	if err := h.svc.RemoveRepoRole(c.Request().Context(), repoID, subject); err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// ---- Local User Management (admin) ----

func (h *Handler) ListUsers(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}
	users, err := h.svc.ListUsers(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}
	if users == nil {
		users = []store.User{}
	}
	return c.JSON(http.StatusOK, map[string]any{"users": users})
}

func (h *Handler) CreateUser(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		IsAdmin     bool   `json:"is_admin"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	user, err := h.svc.RegisterUser(c.Request().Context(), req.Username, req.Password, req.DisplayName, req.Email, req.IsAdmin)
	if err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusCreated, user)
}

func (h *Handler) UpdateUser(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}
	id := strings.TrimSpace(c.Param("id"))
	var req struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		IsAdmin     bool   `json:"is_admin"`
		Disabled    bool   `json:"disabled"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	user, err := h.svc.UpdateUser(c.Request().Context(), id, req.DisplayName, req.Email, req.IsAdmin, req.Disabled)
	if err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, user)
}

func (h *Handler) ResetUserPassword(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}
	id := strings.TrimSpace(c.Param("id"))
	var req struct {
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if err := h.svc.ResetUserPassword(c.Request().Context(), id, req.Password); err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) DeleteUserByID(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}
	if err := h.svc.DeleteUser(c.Request().Context(), c.Param("id")); err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// ---- Auth Config Management (admin) ----

func (h *Handler) ListAuthConfigs(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}
	configs, err := h.svc.ListAuthConfigs(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}
	if configs == nil {
		configs = []service.AuthConfigView{}
	}
	return c.JSON(http.StatusOK, map[string]any{"configs": configs})
}

func (h *Handler) GetAuthConfig(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}
	providerType := strings.TrimSpace(c.Param("type"))
	view, err := h.svc.GetAuthConfig(c.Request().Context(), providerType)
	if err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, view)
}

func (h *Handler) SaveAuthConfig(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}
	providerType := strings.TrimSpace(c.Param("type"))
	var req struct {
		Enabled bool            `json:"enabled"`
		Config  json.RawMessage `json:"config"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if err := h.svc.SaveAuthConfig(c.Request().Context(), providerType, req.Enabled, req.Config); err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) DeleteAuthConfig(c echo.Context) error {
	if err := h.requireAdmin(c); err != nil {
		return err
	}
	providerType := strings.TrimSpace(c.Param("type"))
	if err := h.svc.DeleteAuthConfig(c.Request().Context(), providerType); err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) requireAdmin(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	if !claims.IsAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "admin only")
	}
	return nil
}
