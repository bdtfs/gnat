package runner

import (
	"time"

	"github.com/bdtfs/gnat/internal/models"
)

func NewStats() *models.Stats {
	return &models.Stats{
		StatusCodes: make(map[int]*uint64),
		Latencies:   make([]time.Duration, 0, 10000),
		Errors:      make([]string, 0),
	}
}
