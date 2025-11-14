package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bdtfs/gnat/internal/models"
	"github.com/bdtfs/gnat/internal/runner"
	repository "github.com/bdtfs/gnat/internal/storage/memory"
)

type Service struct {
	repo   *repository.Repository
	runner *runner.Runner
}

func New(repo *repository.Repository, runner *runner.Runner) *Service {
	return &Service{
		repo:   repo,
		runner: runner,
	}
}

func (s *Service) CreateSetup(name, description, method, url string, body []byte, headers map[string]string, rps int, duration time.Duration) (*models.Setup, error) {
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}

	if rps <= 0 {
		return nil, fmt.Errorf("rps must be greater than 0")
	}

	if duration <= 0 {
		return nil, fmt.Errorf("duration must be greater than 0")
	}

	setup := models.NewSetup(name, description, method, url, body, headers, rps, duration)

	if err := s.repo.CreateSetup(setup); err != nil {
		return nil, fmt.Errorf("create setup: %w", err)
	}

	return setup, nil
}

func (s *Service) GetSetup(id string) (*models.Setup, error) {
	return s.repo.GetSetup(id)
}

func (s *Service) ListSetups() []*models.Setup {
	return s.repo.ListSetups()
}

func (s *Service) UpdateSetup(setup *models.Setup) error {
	if _, err := s.repo.GetSetup(setup.ID); err != nil {
		return fmt.Errorf("setup not found: %w", err)
	}

	setup.UpdatedAt = time.Now()
	return s.repo.UpdateSetup(setup)
}

func (s *Service) DeleteSetup(id string) error {
	return s.repo.DeleteSetup(id)
}

func (s *Service) StartRun(ctx context.Context, setupID string) (*models.Run, error) {
	return s.runner.StartRun(ctx, setupID)
}

func (s *Service) GetRun(id string) (*models.Run, error) {
	return s.repo.GetRun(id)
}

func (s *Service) ListRuns() []*models.Run {
	return s.repo.ListRuns()
}

func (s *Service) ListRunsBySetup(setupID string) []*models.Run {
	return s.repo.ListRunsBySetup(setupID)
}

func (s *Service) CancelRun(runID string) error {
	return s.runner.CancelRun(runID)
}

func (s *Service) GetActiveRuns() []string {
	return s.runner.GetActiveRuns()
}
