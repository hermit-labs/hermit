package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const configKeyProxySync = "proxy_sync"

// ProxySyncConfig is the database-backed proxy sync configuration.
type ProxySyncConfig struct {
	PageSize    int `json:"page_size"`
	Concurrency int `json:"concurrency"`
}

func (c ProxySyncConfig) PageSizeOrDefault() int {
	if c.PageSize <= 0 {
		return 100
	}
	return c.PageSize
}

func (c ProxySyncConfig) ConcurrencyOrDefault() int {
	if c.Concurrency <= 0 {
		return 4
	}
	return c.Concurrency
}

func (s *Service) GetProxySyncConfig(ctx context.Context) (ProxySyncConfig, error) {
	raw, err := s.store.GetSystemConfig(ctx, configKeyProxySync)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ProxySyncConfig{
				PageSize:    100,
				Concurrency: 4,
			}, nil
		}
		return ProxySyncConfig{}, err
	}
	var cfg ProxySyncConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ProxySyncConfig{}, fmt.Errorf("parse proxy sync config: %w", err)
	}
	return cfg, nil
}

func (s *Service) SaveProxySyncConfig(ctx context.Context, cfg ProxySyncConfig) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.store.UpsertSystemConfig(ctx, configKeyProxySync, raw)
}

// SeedProxySyncConfig writes the proxy sync config into the DB only if it doesn't exist yet.
func (s *Service) SeedProxySyncConfig(ctx context.Context, cfg ProxySyncConfig) error {
	_, err := s.store.GetSystemConfig(ctx, configKeyProxySync)
	if err == nil {
		return nil
	}
	if err != pgx.ErrNoRows {
		return err
	}
	raw, _ := json.Marshal(cfg)
	return s.store.UpsertSystemConfig(ctx, configKeyProxySync, raw)
}
