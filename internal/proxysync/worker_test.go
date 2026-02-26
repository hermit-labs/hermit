package proxysync

import (
	"context"
	"io"
	"log"
	"sync/atomic"
	"testing"
	"time"
)

type stubRunner struct {
	calls atomic.Int64
}

func (s *stubRunner) Run(_ context.Context, _ int) (Summary, error) {
	s.calls.Add(1)
	return Summary{}, nil
}

func TestWorker_RunOnceWhenNoInterval(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{}
	worker := NewWorker(runner, WorkerConfig{
		Enabled:      true,
		StartupDelay: 0,
		Interval:     0,
		PageSize:     10,
	}, log.New(io.Discard, "", 0))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	worker.Run(ctx)

	if runner.calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", runner.calls.Load())
	}
}

func TestWorker_RunRepeatedlyWithInterval(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{}
	worker := NewWorker(runner, WorkerConfig{
		Enabled:      true,
		StartupDelay: 0,
		Interval:     15 * time.Millisecond,
		PageSize:     10,
	}, log.New(io.Discard, "", 0))

	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Millisecond)
	defer cancel()
	worker.Run(ctx)

	if runner.calls.Load() < 2 {
		t.Fatalf("calls = %d, want >= 2", runner.calls.Load())
	}
}
