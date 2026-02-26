package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hermit/internal/auth"
	"hermit/internal/config"
	"hermit/internal/db"
	"hermit/internal/extauth"
	"hermit/internal/httpapi"
	"hermit/internal/proxysync"
	"hermit/internal/service"
	"hermit/internal/storage"
	"hermit/internal/store"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

	blobStore, err := initBlobStorage(ctx, cfg)
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
		if err := svc.BootstrapDefaults(ctx, cfg.AdminUsername, cfg.AdminPassword); err != nil {
			log.Fatalf("bootstrap defaults: %v", err)
		}
		if cfg.AdminPassword != "" {
			log.Printf("initial admin user ensured (username: %s)", cfg.AdminUsername)
		}

		seedAuthConfigs(ctx, svc, cfg)
		seedProxySyncConfig(ctx, svc, cfg)
	}

	// Proxy sync: always create runner/trigger, worker reads enabled flag from DB.
	factory := proxysync.NewAbstractFactory(
		proxysync.FactoryDeps{
			HTTPClient:      &http.Client{Timeout: cfg.ProxyTimeout},
			VersionCacher:   svc,
			SyncConcurrency: cfg.ProxySyncConcurrency,
		},
		proxysync.NewClawHubBuilder(),
	)
	runner := proxysync.NewRunner(svc, factory)
	configProvider := &syncConfigAdapter{svc: svc}

	worker := proxysync.NewWorker(
		runner,
		proxysync.WorkerConfig{
			Enabled:      true,
			StartupDelay: cfg.ProxySyncDelay,
			Interval:     cfg.ProxySyncInterval,
			PageSize:     cfg.ProxySyncPageSize,
		},
		configProvider,
		log.Default(),
	)
	go worker.Run(ctx)

	syncTrigger := httpapi.NewSyncTrigger(runner, svc, cfg.ProxySyncPageSize, log.Default())

	authn := auth.NewAuthenticator(pool, cfg.AdminToken)

	api := httpapi.New(cfg, svc, authn, syncTrigger, cfg.WebDir)
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

// syncConfigAdapter bridges service.Service to proxysync.ConfigProvider.
type syncConfigAdapter struct {
	svc *service.Service
}

func (a *syncConfigAdapter) GetWorkerConfig(ctx context.Context) (proxysync.WorkerConfig, error) {
	cfg, err := a.svc.GetProxySyncConfig(ctx)
	if err != nil {
		return proxysync.WorkerConfig{}, err
	}
	return proxysync.WorkerConfig{
		Enabled:      cfg.Enabled,
		StartupDelay: cfg.Delay(),
		Interval:     cfg.Interval(),
		PageSize:     cfg.PageSizeOrDefault(),
	}, nil
}

func seedAuthConfigs(ctx context.Context, svc *service.Service, cfg config.Config) {
	if cfg.LDAPEnabled {
		ldapCfg := extauth.LDAPConfig{
			URL:          cfg.LDAPURL,
			BaseDN:       cfg.LDAPBaseDN,
			BindDN:       cfg.LDAPBindDN,
			BindPassword: cfg.LDAPBindPassword,
			UserFilter:   cfg.LDAPUserFilter,
			UserAttr:     cfg.LDAPUserAttr,
			DisplayAttr:  cfg.LDAPDisplayAttr,
			StartTLS:     cfg.LDAPStartTLS,
			SkipVerify:   cfg.LDAPSkipVerify,
			AdminGroups:  cfg.LDAPAdminGroups,
		}
		raw, _ := json.Marshal(ldapCfg)
		if err := svc.SeedAuthConfigFromEnv(ctx, service.ProviderTypeLDAP, true, raw); err != nil {
			log.Printf("warning: seed LDAP config: %v", err)
		} else {
			log.Printf("LDAP config seeded from env vars")
		}
	}

}

func seedProxySyncConfig(ctx context.Context, svc *service.Service, cfg config.Config) {
	psc := service.ProxySyncConfig{
		Enabled:     cfg.ProxySyncEnabled,
		IntervalStr: fmt.Sprintf("%s", cfg.ProxySyncInterval),
		DelayStr:    fmt.Sprintf("%s", cfg.ProxySyncDelay),
		PageSize:    cfg.ProxySyncPageSize,
		Concurrency: cfg.ProxySyncConcurrency,
	}
	if err := svc.SeedProxySyncConfig(ctx, psc); err != nil {
		log.Printf("warning: seed proxy sync config: %v", err)
	} else {
		log.Printf("proxy sync config seeded from env vars (enabled=%v)", cfg.ProxySyncEnabled)
	}
}

func initBlobStorage(ctx context.Context, cfg config.Config) (storage.BlobStorage, error) {
	switch cfg.StorageBackend {
	case "s3":
		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("S3_BUCKET is required when STORAGE_BACKEND=s3")
		}
		opts := []func(*awsconfig.LoadOptions) error{
			awsconfig.WithRegion(cfg.S3Region),
		}
		if cfg.S3AccessKeyID != "" {
			opts = append(opts, awsconfig.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(cfg.S3AccessKeyID, cfg.S3SecretAccessKey, ""),
			))
		}
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("load AWS config: %w", err)
		}

		s3Opts := []func(*s3.Options){}
		if cfg.S3Endpoint != "" {
			s3Opts = append(s3Opts, func(o *s3.Options) {
				o.BaseEndpoint = &cfg.S3Endpoint
				o.UsePathStyle = cfg.S3ForcePathStyle
			})
		}
		client := s3.NewFromConfig(awsCfg, s3Opts...)

		log.Printf("storage backend: s3 (bucket=%s, endpoint=%s)", cfg.S3Bucket, cfg.S3Endpoint)
		return storage.NewS3BlobStore(storage.S3Options{
			Client: client,
			Bucket: cfg.S3Bucket,
			Prefix: cfg.S3Prefix,
		}), nil

	default:
		log.Printf("storage backend: local (%s)", cfg.StorageRoot)
		return storage.NewLocalBlobStore(cfg.StorageRoot)
	}
}
