package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bdtfs/gnat/internal/models"
	repository "github.com/bdtfs/gnat/internal/storage/memory"
)

type Runner struct {
	repo         *repository.Repository
	logger       *slog.Logger
	collector    *Collector
	activeRuns   map[string]context.CancelFunc
	activeRunsMu sync.RWMutex
}

func New(repo *repository.Repository, logger *slog.Logger, collector *Collector) *Runner {
	return &Runner{
		repo:       repo,
		logger:     logger,
		collector:  collector,
		activeRuns: make(map[string]context.CancelFunc),
	}
}

func (r *Runner) StartRun(ctx context.Context, setupID string) (*models.Run, error) {
	setup, err := r.repo.GetSetup(setupID)
	if err != nil {
		return nil, fmt.Errorf("get setup: %w", err)
	}

	if setup.Status != models.SetupStatusActive {
		return nil, fmt.Errorf("setup is not active")
	}

	run := models.NewRun(setupID)

	if err = r.repo.CreateRun(run); err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}

	root := context.WithoutCancel(ctx)
	runCtx, cancel := context.WithCancel(root)

	r.activeRunsMu.Lock()
	r.activeRuns[run.ID] = cancel
	r.activeRunsMu.Unlock()

	r.logger.Info("run starting", "run_id", run.ID)

	go r.execute(runCtx, run, setup)

	return run, nil
}

func (r *Runner) execute(ctx context.Context, run *models.Run, setup *models.Setup) {
	defer func() {
		r.activeRunsMu.Lock()
		delete(r.activeRuns, run.ID)
		r.activeRunsMu.Unlock()
	}()

	run.Status = models.RunStatusRunning

	start := time.Now()
	err := r.runLoop(ctx, run, setup)
	duration := time.Since(start)

	run.EndedAt = time.Now()

	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		run.Status = models.RunStatusCancelled
	case err != nil:
		run.Status = models.RunStatusFailed
		run.Error = err.Error()
	default:
		run.Status = models.RunStatusCompleted
	}

	r.logger.Info(
		"run finished",
		"run_id", run.ID,
		"setup_id", setup.ID,
		"status", run.Status,
		"duration", duration,
		"total_requests", run.Stats.TotalRequests,
		"success_requests", run.Stats.SuccessRequests,
		"failed_requests", run.Stats.FailedRequests,
	)

	if err = r.repo.UpdateRun(run); err != nil {
		r.logger.Error("update run failed", "run_id", run.ID, "error", err)
	}
}

func (r *Runner) CancelRun(runID string) error {
	r.activeRunsMu.RLock()
	cancel, ok := r.activeRuns[runID]
	r.activeRunsMu.RUnlock()

	if !ok {
		return fmt.Errorf("run %s is not active", runID)
	}

	cancel()
	return nil
}

func (r *Runner) GetActiveRuns() []string {
	r.activeRunsMu.RLock()
	defer r.activeRunsMu.RUnlock()

	out := make([]string, 0, len(r.activeRuns))
	for id := range r.activeRuns {
		out = append(out, id)
	}

	return out
}
