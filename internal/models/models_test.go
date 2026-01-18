package models

import (
	"testing"
	"time"
)

func TestNewSetup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupName   string
		description string
		method      string
		url         string
		body        []byte
		headers     map[string]string
		rps         int
		duration    time.Duration
	}{
		{
			name:        "basic GET setup",
			setupName:   "Test Setup",
			description: "A test setup",
			method:      "GET",
			url:         "https://example.com",
			body:        nil,
			headers:     nil,
			rps:         100,
			duration:    30 * time.Second,
		},
		{
			name:        "POST with body and headers",
			setupName:   "POST Setup",
			description: "A POST test setup",
			method:      "POST",
			url:         "https://api.example.com/data",
			body:        []byte(`{"key": "value"}`),
			headers:     map[string]string{"Content-Type": "application/json"},
			rps:         50,
			duration:    1 * time.Minute,
		},
		{
			name:        "empty values",
			setupName:   "",
			description: "",
			method:      "",
			url:         "",
			body:        nil,
			headers:     nil,
			rps:         0,
			duration:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			setup := NewSetup(tt.setupName, tt.description, tt.method, tt.url, tt.body, tt.headers, tt.rps, tt.duration)

			if setup.ID == "" {
				t.Error("expected non-empty ID")
			}
			if setup.Name != tt.setupName {
				t.Errorf("expected Name %q, got %q", tt.setupName, setup.Name)
			}
			if setup.Description != tt.description {
				t.Errorf("expected Description %q, got %q", tt.description, setup.Description)
			}
			if setup.Method != tt.method {
				t.Errorf("expected Method %q, got %q", tt.method, setup.Method)
			}
			if setup.URL != tt.url {
				t.Errorf("expected URL %q, got %q", tt.url, setup.URL)
			}
			if setup.RPS != tt.rps {
				t.Errorf("expected RPS %d, got %d", tt.rps, setup.RPS)
			}
			if setup.Duration != tt.duration {
				t.Errorf("expected Duration %v, got %v", tt.duration, setup.Duration)
			}
			if setup.Status != SetupStatusActive {
				t.Errorf("expected Status %q, got %q", SetupStatusActive, setup.Status)
			}
			if setup.CreatedAt.IsZero() {
				t.Error("expected non-zero CreatedAt")
			}
			if setup.UpdatedAt.IsZero() {
				t.Error("expected non-zero UpdatedAt")
			}
		})
	}
}

func TestNewRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setupID string
	}{
		{
			name:    "valid setup ID",
			setupID: "setup-123",
		},
		{
			name:    "empty setup ID",
			setupID: "",
		},
		{
			name:    "UUID setup ID",
			setupID: "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			run := NewRun(tt.setupID)

			if run.ID == "" {
				t.Error("expected non-empty ID")
			}
			if run.SetupID != tt.setupID {
				t.Errorf("expected SetupID %q, got %q", tt.setupID, run.SetupID)
			}
			if run.Status != RunStatusPending {
				t.Errorf("expected Status %q, got %q", RunStatusPending, run.Status)
			}
			if run.StartedAt.IsZero() {
				t.Error("expected non-zero StartedAt")
			}
		})
	}
}

func TestSetupStatus(t *testing.T) {
	t.Parallel()

	if SetupStatusActive != "active" {
		t.Errorf("expected SetupStatusActive to be 'active', got %q", SetupStatusActive)
	}
	if SetupStatusInactive != "inactive" {
		t.Errorf("expected SetupStatusInactive to be 'inactive', got %q", SetupStatusInactive)
	}
}

func TestRunStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status   RunStatus
		expected string
	}{
		{RunStatusPending, "pending"},
		{RunStatusRunning, "running"},
		{RunStatusCompleted, "completed"},
		{RunStatusFailed, "failed"},
		{RunStatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, tt.status)
		}
	}
}

func TestNewSetup_UniqueIDs(t *testing.T) {
	t.Parallel()

	setup1 := NewSetup("test1", "", "GET", "http://example.com", nil, nil, 100, time.Second)
	setup2 := NewSetup("test2", "", "GET", "http://example.com", nil, nil, 100, time.Second)

	if setup1.ID == setup2.ID {
		t.Error("expected different IDs for different setups")
	}
}

func TestNewRun_UniqueIDs(t *testing.T) {
	t.Parallel()

	run1 := NewRun("setup-1")
	run2 := NewRun("setup-1")

	if run1.ID == run2.ID {
		t.Error("expected different IDs for different runs")
	}
}
