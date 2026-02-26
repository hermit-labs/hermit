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
	v1.GET("/search", a.handler.Search)
	v1.GET("/skills", a.handler.ListSkills)
	v1.GET("/skills/:slug", a.handler.GetSkill)
	v1.GET("/skills/:slug/versions", a.handler.ListVersions)
	v1.GET("/skills/:slug/versions/:version", a.handler.GetVersion)
	v1.GET("/skills/:slug/file", a.handler.GetSkillFile)
	v1.GET("/resolve", a.handler.Resolve)
	v1.GET("/download", a.handler.Download)
}

func (a *API) registerAuthV1Routes(v1 *echo.Group) {
	v1Auth := v1.Group("")
	v1Auth.Use(a.auth.Middleware)
	v1Auth.GET("/whoami", a.handler.Whoami)
	v1Auth.POST("/skills", a.handler.PublishSkill)
	v1Auth.DELETE("/skills/:slug", a.handler.DeleteSkill)
	v1Auth.POST("/skills/:slug/undelete", a.handler.UndeleteSkill)
}

func (a *API) registerInternalRoutes(e *echo.Echo) {
	internal := e.Group("/api/internal")
	internal.Use(a.auth.Middleware)
	internal.POST("/tokens", a.handler.CreateToken)
}
