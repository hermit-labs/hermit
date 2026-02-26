package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hermit/internal/auth"
	"hermit/internal/config"
	"hermit/internal/db"
	"hermit/internal/httpapi"
	"hermit/internal/proxysync"
	"hermit/internal/service"
	"hermit/internal/storage"
	"hermit/internal/store"
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	blobStore, err := storage.NewBlobStore(cfg.StorageRoot)
	if err != nil {
		log.Fatalf("init storage: %v", err)
	}

	st := store.New(pool)
	svc := service.New(
		st,
		blobStore,
		cfg.ProxyTimeout,
		cfg.ProxyNegativeTTL,
		service.Defaults{
			HostedRepo:     cfg.DefaultHostedRepo,
			GroupRepo:      cfg.DefaultGroupRepo,
			ProxyRepo:      cfg.DefaultProxyRepo,
			ProxyUpstreams: cfg.ProxyUpstreamURLs,
		},
	)
	if cfg.BootstrapDefaults {
		if err := svc.BootstrapDefaults(ctx, "admin"); err != nil {
			log.Fatalf("bootstrap defaults: %v", err)
		}
	}
	if cfg.ProxySyncEnabled {
		factory := proxysync.NewAbstractFactory(
			proxysync.FactoryDeps{
				HTTPClient:      &http.Client{Timeout: cfg.ProxyTimeout},
				VersionCacher:   svc,
				SyncConcurrency: cfg.ProxySyncConcurrency,
			},
			proxysync.NewClawHubBuilder(),
		)
		runner := proxysync.NewRunner(svc, factory)
		worker := proxysync.NewWorker(
			runner,
			proxysync.WorkerConfig{
				Enabled:      true,
				StartupDelay: cfg.ProxySyncDelay,
				Interval:     cfg.ProxySyncInterval,
				PageSize:     cfg.ProxySyncPageSize,
			},
			log.Default(),
		)
		go worker.Run(ctx)
	}

	authn := auth.NewAuthenticator(pool, cfg.AdminToken)
	api := httpapi.New(cfg, svc, authn)
	echoServer := api.NewEcho()

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      echoServer,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	}

	go func() {
		log.Printf("listening on %s", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
		os.Exit(1)
	}
}
