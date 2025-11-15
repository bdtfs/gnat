package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bdtfs/gnat/internal/models"
	"github.com/bdtfs/gnat/internal/stats"
	repository "github.com/bdtfs/gnat/internal/storage/memory"
)

type Runner struct {
	repo         *repository.Repository
	logger       *slog.Logger
	activeRuns   map[string]context.CancelFunc
	activeRunsMu sync.RWMutex
}

func New(repo *repository.Repository, logger *slog.Logger) *Runner {
	return &Runner{
		repo:       repo,
		logger:     logger,
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

	runCtx := context.WithoutCancel(ctx)
	cancellableRunCtx, cancel := context.WithCancel(runCtx)

	r.activeRunsMu.Lock()
	r.activeRuns[run.ID] = cancel
	r.activeRunsMu.Unlock()

	r.logger.Info(
		"run execution starting",
		"run_id", run.ID,
		"setup_id", setupID,
		"url", setup.URL,
		"rps", setup.RPS,
		"duration", setup.Duration,
	)

	go r.executeRun(cancellableRunCtx, run, setup)

	return run, nil
}

func (r *Runner) executeRun(ctx context.Context, run *models.Run, setup *models.Setup) {
	defer func() {
		r.activeRunsMu.Lock()
		delete(r.activeRuns, run.ID)
		r.activeRunsMu.Unlock()
	}()

	r.logger.Info("executing attack", "run_id", run.ID, "url", setup.URL, "rps", setup.RPS)
	run.Status = models.RunStatusRunning

	attackStats, err := r.run(ctx, nil, setup.Method, setup.URL, setup.RPS, setup.Duration, setup.Body)

	run.EndedAt = time.Now()

	if err != nil {
		switch {
		case errors.Is(ctx.Err(), context.Canceled):
			run.Status = models.RunStatusCancelled
			r.logger.Info("run cancelled", "run_id", run.ID)
		default:
			run.Status = models.RunStatusFailed
			run.Error = err.Error()
			r.logger.Error("run failed", "run_id", run.ID, "error", err)
		}
	}

	if attackStats == nil {
		panic(fmt.Errorf("no error, but stats is nil"))
	}

	run.Status = models.RunStatusCompleted
	run.Stats = &models.RunStats{
		Total:       attackStats.Total(),
		Success:     attackStats.Success(),
		Failed:      attackStats.Failed(),
		AvgLatency:  attackStats.AvgLatency(),
		SuccessRate: r.calculateSuccessRate(attackStats),
	}

	r.logger.Info("run completed",
		"run_id", run.ID,
		"total", run.Stats.Total,
		"success", run.Stats.Success,
		"failed", run.Stats.Failed,
		"avg_latency", run.Stats.AvgLatency,
		"success_rate", run.Stats.SuccessRate)

	if err = r.repo.UpdateRun(run); err != nil {
		r.logger.Error("failed to update run", "run_id", run.ID, "error", err)
	}
}

func (r *Runner) calculateSuccessRate(s *stats.Stats) float64 {
	total := s.Total()
	if total == 0 {
		return 0
	}
	return float64(s.Success()) / float64(total) * 100
}

func (r *Runner) CancelRun(runID string) error {
	r.activeRunsMu.RLock()
	cancel, exists := r.activeRuns[runID]
	r.activeRunsMu.RUnlock()

	if !exists {
		return fmt.Errorf("run %s is not active", runID)
	}

	r.logger.Info("cancelling run", "run_id", runID)
	cancel()
	return nil
}

func (r *Runner) GetActiveRuns() []string {
	r.activeRunsMu.RLock()
	defer r.activeRunsMu.RUnlock()

	runIDs := make([]string, 0, len(r.activeRuns))
	for id := range r.activeRuns {
		runIDs = append(runIDs, id)
	}

	return runIDs
}
