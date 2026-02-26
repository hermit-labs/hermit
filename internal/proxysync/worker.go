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

type WorkerConfig struct {
	Enabled      bool
	StartupDelay time.Duration
	Interval     time.Duration
	PageSize     int
}

type Worker struct {
	runner workerRunner
	cfg    WorkerConfig
	logger *log.Logger
}

func NewWorker(runner workerRunner, cfg WorkerConfig, logger *log.Logger) *Worker {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	if cfg.PageSize <= 0 {
		cfg.PageSize = 100
	}
	if cfg.StartupDelay < 0 {
		cfg.StartupDelay = 0
	}
	if cfg.Interval < 0 {
		cfg.Interval = 0
	}

	return &Worker{
		runner: runner,
		cfg:    cfg,
		logger: logger,
	}
}

func (w *Worker) Run(ctx context.Context) {
	if !w.cfg.Enabled || w.runner == nil {
		return
	}
	if w.cfg.StartupDelay > 0 {
		timer := time.NewTimer(w.cfg.StartupDelay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}

	w.runOnce(ctx)
	if w.cfg.Interval <= 0 {
		return
	}

	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) {
	start := time.Now()
	summary, err := w.runner.Run(ctx, w.cfg.PageSize)
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
