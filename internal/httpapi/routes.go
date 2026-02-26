package httpapi

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

func (a *API) registerRoutes(e *echo.Echo) {
	e.GET("/.well-known/clawhub.json", a.handler.WellKnown)
	e.GET("/.well-known/clawdhub.json", a.handler.WellKnown)

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"ok":        true,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	v1 := e.Group("/api/v1")
	a.registerPublicV1Routes(v1)
	a.registerAuthV1Routes(v1)
	a.registerInternalRoutes(e)
}

func (a *API) registerPublicV1Routes(v1 *echo.Group) {
	// Apply optional auth so public endpoints can identify the caller
	// for repository-level access filtering.
	v1.Use(a.auth.OptionalMiddleware)

	v1.GET("/search", a.handler.Search)
	v1.GET("/skills", a.handler.ListSkills)
	v1.GET("/skills/:slug", a.handler.GetSkill)
	v1.GET("/skills/:slug/versions", a.handler.ListVersions)
	v1.GET("/skills/:slug/versions/:version", a.handler.GetVersion)
	v1.GET("/skills/:slug/file", a.handler.GetSkillFile)
	v1.GET("/resolve", a.handler.Resolve)
	v1.GET("/download", a.handler.Download)

	// Auth endpoints (public, no token required)
	v1.GET("/auth/providers", a.handler.AuthProviders)
	v1.POST("/auth/login", a.handler.LocalLogin)
	v1.POST("/auth/ldap", a.handler.LDAPLogin)
}

func (a *API) registerAuthV1Routes(v1 *echo.Group) {
	v1Auth := v1.Group("")
	v1Auth.Use(a.auth.Middleware)
	v1Auth.GET("/whoami", a.handler.Whoami)
	v1Auth.POST("/skills", a.handler.PublishSkill)
	v1Auth.DELETE("/skills/:slug", a.handler.DeleteSkill)
	v1Auth.POST("/skills/:slug/undelete", a.handler.UndeleteSkill)

	// Personal Access Tokens (user self-service)
	v1Auth.GET("/tokens", a.handler.ListMyTokens)
	v1Auth.POST("/tokens", a.handler.CreateMyToken)
	v1Auth.DELETE("/tokens/:tokenId", a.handler.RevokeMyToken)
}

func (a *API) registerInternalRoutes(e *echo.Echo) {
	internal := e.Group("/api/internal")
	internal.Use(a.auth.Middleware)
	internal.POST("/tokens", a.handler.AdminCreateToken)

	// Dashboard & sync
	internal.GET("/stats", a.handler.GetDashboardStats)
	internal.GET("/sync-sources", a.handler.ListSyncSources)
	internal.POST("/sync-sources", a.handler.AddSyncSource)
	internal.DELETE("/sync-sources/:id", a.handler.RemoveSyncSource)
	internal.PATCH("/sync-sources/:id", a.handler.ToggleSyncSource)
	internal.POST("/sync", a.handler.TriggerSync)
	internal.GET("/sync/status", a.handler.GetSyncStatus)
	internal.GET("/sync/config", a.handler.GetProxySyncConfig)
	internal.PUT("/sync/config", a.handler.SaveProxySyncConfig)

	// RBAC management
	internal.GET("/rbac/members", a.handler.ListAllMembers)
	internal.GET("/rbac/repos/:id/members", a.handler.ListMembers)
	internal.POST("/rbac/repos/:id/members", a.handler.AssignMember)
	internal.DELETE("/rbac/repos/:id/members/:subject", a.handler.RemoveMember)

	// User management (admin)
	internal.GET("/users", a.handler.ListUsers)
	internal.POST("/users", a.handler.CreateUser)
	internal.PATCH("/users/:id", a.handler.UpdateUser)
	internal.POST("/users/:id/reset-password", a.handler.ResetUserPassword)
	internal.DELETE("/users/:id", a.handler.DeleteUserByID)

	// Auth provider config (admin)
	internal.GET("/auth-configs", a.handler.ListAuthConfigs)
	internal.GET("/auth-configs/:type", a.handler.GetAuthConfig)
	internal.PUT("/auth-configs/:type", a.handler.SaveAuthConfig)
	internal.DELETE("/auth-configs/:type", a.handler.DeleteAuthConfig)
}
