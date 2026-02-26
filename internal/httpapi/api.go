package httpapi

import (
	"net/http"

	"hermit/internal/auth"
	"hermit/internal/config"
	"hermit/internal/httpapi/handlers"
	"hermit/internal/httpapi/middlewares"
	"hermit/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type API struct {
	cfg     config.Config
	auth    *auth.Authenticator
	handler *handlers.Handler
}

func New(cfg config.Config, svc *service.Service, authn *auth.Authenticator) *API {
	return &API{
		cfg:     cfg,
		auth:    authn,
		handler: handlers.New(cfg, svc, authn),
	}
}

func (a *API) NewEcho() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.RequestLogger())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: a.cfg.CORSAllowedOrigins,
		AllowMethods: []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPost,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderAccept,
			echo.HeaderContentType,
			echo.HeaderAuthorization,
			"X-API-Token",
		},
		ExposeHeaders: []string{
			"RateLimit-Limit",
			"RateLimit-Remaining",
			"RateLimit-Reset",
			"X-RateLimit-Limit",
			"X-RateLimit-Remaining",
			"X-RateLimit-Reset",
			"Retry-After",
		},
		MaxAge: 600,
	}))
	e.Use(middlewares.NewRateLimitMiddleware(a.auth))

	a.registerRoutes(e)
	return e
}
