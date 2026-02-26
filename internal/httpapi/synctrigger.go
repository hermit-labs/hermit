package httpapi

import (
	"context"
	"log"
	"sync"

	"hermit/internal/httpapi/handlers"
	"hermit/internal/proxysync"
	"hermit/internal/service"
)

type SyncTrigger struct {
	runner       *proxysync.Runner
	svc          *service.Service
	fallbackPage int
	logger       *log.Logger

	mu         sync.Mutex
	running    bool
	lastResult *proxysync.Summary
	lastError  error
}

func NewSyncTrigger(runner *proxysync.Runner, svc *service.Service, fallbackPageSize int, logger *log.Logger) *SyncTrigger {
	return &SyncTrigger{
		runner:       runner,
		svc:          svc,
		fallbackPage: fallbackPageSize,
		logger:       logger,
	}
}

func (st *SyncTrigger) TriggerSync(_ context.Context) (bool, error) {
	st.mu.Lock()
	if st.running {
		st.mu.Unlock()
		return false, nil
	}
	st.running = true
	st.mu.Unlock()

	go func() {
		pageSize := st.fallbackPage
		if st.svc != nil {
			if cfg, err := st.svc.GetProxySyncConfig(context.Background()); err == nil {
				pageSize = cfg.PageSizeOrDefault()
			}
		}

		summary, err := st.runner.Run(context.Background(), pageSize)

		st.mu.Lock()
		st.running = false
		st.lastResult = &summary
		if err != nil {
			st.lastError = err
			st.logger.Printf("manual sync failed: %v", err)
		} else {
			st.lastError = nil
			st.logger.Printf(
				"manual sync finished: repos=%d skills=%d versions=%d cached=%d failed=%d",
				summary.Repositories, summary.Skills, summary.Versions, summary.Cached, summary.Failed,
			)
		}
		st.mu.Unlock()
	}()

	return true, nil
}

func (st *SyncTrigger) Status() handlers.SyncStatus {
	st.mu.Lock()
	defer st.mu.Unlock()

	errStr := ""
	if st.lastError != nil {
		errStr = st.lastError.Error()
	}
	return handlers.SyncStatus{
		Running:    st.running,
		LastResult: st.lastResult,
		LastError:  errStr,
	}
}
