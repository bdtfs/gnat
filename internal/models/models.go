package models

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type SetupStatus string

const (
	SetupStatusActive   SetupStatus = "active"
	SetupStatusInactive SetupStatus = "inactive"
)

type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

type Setup struct {
	ID          string
	Name        string
	Description string
	Method      string
	URL         string
	Body        []byte
	Headers     map[string]string
	RPS         int
	Duration    time.Duration
	Status      SetupStatus
	HTTPConfig  map[string]interface{}
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Run struct {
	ID        string
	SetupID   string
	Status    RunStatus
	StartedAt time.Time
	EndedAt   time.Time
	Error     string
	Stats     *Stats
}

type Stats struct {
	TotalRequests   uint64
	SuccessRequests uint64
	FailedRequests  uint64
	TotalBytesRead  uint64

	StatusCodes map[int]*uint64
	StatusMu    sync.RWMutex

	Latencies   []time.Duration
	LatenciesMu sync.Mutex

	TotalLatency time.Duration
	LatencyMu    sync.Mutex

	Errors   []string
	ErrorsMu sync.RWMutex
}

func NewSetup(name, description, method, url string, body []byte, headers map[string]string, rps int, duration time.Duration) *Setup {
	now := time.Now()
	return &Setup{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Method:      method,
		URL:         url,
		Body:        body,
		Headers:     headers,
		RPS:         rps,
		Duration:    duration,
		Status:      SetupStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func NewRun(setupID string) *Run {
	return &Run{
		ID:        uuid.New().String(),
		SetupID:   setupID,
		Status:    RunStatusPending,
		StartedAt: time.Now(),
	}
}
