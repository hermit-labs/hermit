package handlers

import (
	"context"

	"hermit/internal/auth"
	"hermit/internal/config"
	"hermit/internal/proxysync"
	"hermit/internal/service"
)

type SyncTriggerer interface {
	TriggerSync(ctx context.Context) (bool, error)
	Status() SyncStatus
}

type SyncStatus struct {
	Running    bool               `json:"running"`
	LastResult *proxysync.Summary `json:"lastResult"`
	LastError  string             `json:"lastError"`
}

type Handler struct {
	cfg         config.Config
	svc         *service.Service
	auth        *auth.Authenticator
	syncTrigger SyncTriggerer
}

func New(
	cfg config.Config,
	svc *service.Service,
	authn *auth.Authenticator,
	syncTrigger SyncTriggerer,
) *Handler {
	return &Handler{
		cfg:         cfg,
		svc:         svc,
		auth:        authn,
		syncTrigger: syncTrigger,
	}
}
