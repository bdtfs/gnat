package memory

import (
	"fmt"
	"sync"

	"github.com/bdtfs/gnat/internal/models"
)

type Repository struct {
	setups map[string]*models.Setup
	runs   map[string]*models.Run
	mu     sync.RWMutex
}

func New() *Repository {
	return &Repository{
		setups: make(map[string]*models.Setup),
		runs:   make(map[string]*models.Run),
	}
}

func (r *Repository) CreateSetup(setup *models.Setup) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.setups[setup.ID]; exists {
		return fmt.Errorf("setup with id %s already exists", setup.ID)
	}

	r.setups[setup.ID] = setup
	return nil
}

func (r *Repository) GetSetup(id string) (*models.Setup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	setup, exists := r.setups[id]
	if !exists {
		return nil, fmt.Errorf("setup with id %s not found", id)
	}

	return setup, nil
}

func (r *Repository) ListSetups() []*models.Setup {
	r.mu.RLock()
	defer r.mu.RUnlock()

	setups := make([]*models.Setup, 0, len(r.setups))
	for _, setup := range r.setups {
		setups = append(setups, setup)
	}

	return setups
}

func (r *Repository) UpdateSetup(setup *models.Setup) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.setups[setup.ID]; !exists {
		return fmt.Errorf("setup with id %s not found", setup.ID)
	}

	r.setups[setup.ID] = setup
	return nil
}

func (r *Repository) DeleteSetup(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.setups[id]; !exists {
		return fmt.Errorf("setup with id %s not found", id)
	}

	delete(r.setups, id)
	return nil
}

func (r *Repository) CreateRun(run *models.Run) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.runs[run.ID]; exists {
		return fmt.Errorf("run with id %s already exists", run.ID)
	}

	r.runs[run.ID] = run
	return nil
}

func (r *Repository) GetRun(id string) (*models.Run, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	run, exists := r.runs[id]
	if !exists {
		return nil, fmt.Errorf("run with id %s not found", id)
	}

	return run, nil
}

func (r *Repository) ListRuns() []*models.Run {
	r.mu.RLock()
	defer r.mu.RUnlock()

	runs := make([]*models.Run, 0, len(r.runs))
	for _, run := range r.runs {
		runs = append(runs, run)
	}

	return runs
}

func (r *Repository) ListRunsBySetup(setupID string) []*models.Run {
	r.mu.RLock()
	defer r.mu.RUnlock()

	runs := make([]*models.Run, 0)
	for _, run := range r.runs {
		if run.SetupID == setupID {
			runs = append(runs, run)
		}
	}

	return runs
}

func (r *Repository) UpdateRun(run *models.Run) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.runs[run.ID]; !exists {
		return fmt.Errorf("run with id %s not found", run.ID)
	}

	r.runs[run.ID] = run
	return nil
}

func (r *Repository) DeleteRun(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.runs[id]; !exists {
		return fmt.Errorf("run with id %s not found", id)
	}

	delete(r.runs, id)
	return nil
}
