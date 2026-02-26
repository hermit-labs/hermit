package handlers

import (
	"hermit/internal/auth"
	"hermit/internal/config"
	"hermit/internal/service"
)

type Handler struct {
	cfg  config.Config
	svc  *service.Service
	auth *auth.Authenticator
}

func New(cfg config.Config, svc *service.Service, authn *auth.Authenticator) *Handler {
	return &Handler{
		cfg:  cfg,
		svc:  svc,
		auth: authn,
	}
}
