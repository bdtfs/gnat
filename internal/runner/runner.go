package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bdtfs/gnat/internal/attack"
	"github.com/bdtfs/gnat/internal/models"
	"github.com/bdtfs/gnat/internal/stats"
	repository "github.com/bdtfs/gnat/internal/storage/memory"
)

type Runner struct {
	repo         *repository.Repository
	activeRuns   map[string]context.CancelFunc
	activeRunsMu sync.RWMutex
}

func New(repo *repository.Repository) *Runner {
	return &Runner{
		repo:       repo,
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
	run.Status = models.RunStatusRunning

	if err := r.repo.CreateRun(run); err != nil {
		return nil, fmt.Errorf("create run: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)

	r.activeRunsMu.Lock()
	r.activeRuns[run.ID] = cancel
	r.activeRunsMu.Unlock()

	go r.executeRun(runCtx, run, setup)

	return run, nil
}

func (r *Runner) executeRun(ctx context.Context, run *models.Run, setup *models.Setup) {
	defer func() {
		r.activeRunsMu.Lock()
		delete(r.activeRuns, run.ID)
		r.activeRunsMu.Unlock()
	}()

	attackCtx := context.Background()
	attackStats, err := attack.Run(attackCtx, nil, setup.Method, setup.URL, setup.RPS, setup.Duration, setup.Body)

	run.EndedAt = time.Now()

	if err != nil {
		run.Status = models.RunStatusFailed
		run.Error = err.Error()
	} else if errors.Is(ctx.Err(), context.Canceled) {
		run.Status = models.RunStatusCancelled
	} else {
		run.Status = models.RunStatusCompleted
		run.Stats = &models.RunStats{
			Total:       attackStats.Total(),
			Success:     attackStats.Success(),
			Failed:      attackStats.Failed(),
			AvgLatency:  attackStats.AvgLatency(),
			SuccessRate: r.calculateSuccessRate(attackStats),
		}
	}

	if err := r.repo.UpdateRun(run); err != nil {
		fmt.Printf("failed to update run: %v\n", err)
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
