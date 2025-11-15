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

	startTime := time.Now()
	err := r.run(ctx, run.ID, nil, setup.Method, setup.URL, setup.RPS, setup.Duration, setup.Body)
	duration := time.Since(startTime)

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
	} else {
		run.Status = models.RunStatusCompleted
	}

	attackStats := r.collector.GetStats(run.ID)
	if attackStats != nil {
		run.Stats = &models.RunStats{
			Total:       attackStats.Total(),
			Success:     attackStats.Success(),
			Failed:      attackStats.Failed(),
			AvgLatency:  attackStats.AvgLatency(),
			MinLatency:  attackStats.MinLatency(),
			MaxLatency:  attackStats.MaxLatency(),
			P50Latency:  attackStats.Percentile(0.50),
			P90Latency:  attackStats.Percentile(0.90),
			P95Latency:  attackStats.Percentile(0.95),
			P99Latency:  attackStats.Percentile(0.99),
			SuccessRate: r.calculateSuccessRate(attackStats),
			RPS:         attackStats.RPS(duration),
			BytesRead:   attackStats.BytesRead(),
			StatusCodes: attackStats.StatusCodeDistribution(),
			Errors:      attackStats.Errors(),
		}

		r.logger.Info("run completed",
			"run_id", run.ID,
			"total", run.Stats.Total,
			"success", run.Stats.Success,
			"failed", run.Stats.Failed,
			"avg_latency", run.Stats.AvgLatency,
			"p95_latency", run.Stats.P95Latency,
			"p99_latency", run.Stats.P99Latency,
			"success_rate", run.Stats.SuccessRate,
			"rps", run.Stats.RPS)
	}

	if err = r.repo.UpdateRun(run); err != nil {
		r.logger.Error("failed to update run", "run_id", run.ID, "error", err)
	}
}

func (r *Runner) calculateSuccessRate(s *Stats) float64 {
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

func (r *Runner) GetLiveStats(runID string) *Stats {
	return r.collector.GetStats(runID)
}

func (r *Runner) Shutdown() {
	r.collector.Shutdown()
}
