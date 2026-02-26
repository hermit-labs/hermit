package proxysync

import (
	"context"
	"io"
	"log"
	"time"
)

type workerRunner interface {
	Run(context.Context, int) (Summary, error)
}

// ConfigProvider dynamically provides worker configuration from the database.
type ConfigProvider interface {
	GetWorkerConfig(ctx context.Context) (WorkerConfig, error)
}

type WorkerConfig struct {
	Enabled      bool
	StartupDelay time.Duration
	Interval     time.Duration
	PageSize     int
}

type Worker struct {
	runner       workerRunner
	configSource ConfigProvider
	fallbackCfg  WorkerConfig
	logger       *log.Logger
}

// NewWorker creates a worker. If configSource is nil, fallbackCfg is used statically.
func NewWorker(runner workerRunner, fallbackCfg WorkerConfig, configSource ConfigProvider, logger *log.Logger) *Worker {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	if fallbackCfg.PageSize <= 0 {
		fallbackCfg.PageSize = 100
	}
	if fallbackCfg.StartupDelay < 0 {
		fallbackCfg.StartupDelay = 0
	}
	if fallbackCfg.Interval < 0 {
		fallbackCfg.Interval = 0
	}

	return &Worker{
		runner:       runner,
		configSource: configSource,
		fallbackCfg:  fallbackCfg,
		logger:       logger,
	}
}

func (w *Worker) getConfig(ctx context.Context) WorkerConfig {
	if w.configSource == nil {
		return w.fallbackCfg
	}
	cfg, err := w.configSource.GetWorkerConfig(ctx)
	if err != nil {
		w.logger.Printf("failed to read sync config from DB, using fallback: %v", err)
		return w.fallbackCfg
	}
	return cfg
}

func (w *Worker) Run(ctx context.Context) {
	cfg := w.getConfig(ctx)
	if !cfg.Enabled || w.runner == nil {
		return
	}
	if cfg.StartupDelay > 0 {
		timer := time.NewTimer(cfg.StartupDelay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}

	w.runOnce(ctx)

	cfg = w.getConfig(ctx)
	if cfg.Interval <= 0 {
		return
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			latest := w.getConfig(ctx)
			if !latest.Enabled {
				w.logger.Printf("proxy sync disabled via config, pausing")
				continue
			}
			// Update ticker if interval changed
			if latest.Interval > 0 && latest.Interval != cfg.Interval {
				ticker.Reset(latest.Interval)
				w.logger.Printf("sync interval updated to %s", latest.Interval)
				cfg.Interval = latest.Interval
			}
			cfg = latest
			w.runOnce(ctx)
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) {
	cfg := w.getConfig(ctx)
	pageSize := cfg.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	start := time.Now()
	summary, err := w.runner.Run(ctx, pageSize)
	if err != nil {
		w.logger.Printf("proxy sync failed after %s: %v", time.Since(start).Round(time.Millisecond), err)
		return
	}
	w.logger.Printf(
		"proxy sync finished in %s: repos=%d skills=%d versions=%d cached=%d failed=%d",
		time.Since(start).Round(time.Millisecond),
		summary.Repositories,
		summary.Skills,
		summary.Versions,
		summary.Cached,
		summary.Failed,
	)
}
