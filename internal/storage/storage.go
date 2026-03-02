package storage

import "github.com/bdtfs/gnat/internal/models"

// Repository defines the interface for data persistence operations.
type Repository interface {
	CreateSetup(setup *models.Setup) error
	GetSetup(id string) (*models.Setup, error)
	ListSetups() []*models.Setup
	UpdateSetup(setup *models.Setup) error
	DeleteSetup(id string) error

	CreateRun(run *models.Run) error
	GetRun(id string) (*models.Run, error)
	ListRuns() []*models.Run
	ListRunsBySetup(setupID string) []*models.Run
	UpdateRun(run *models.Run) error
	DeleteRun(id string) error
}
