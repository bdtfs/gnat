package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bdtfs/gnat/internal/models"
	"github.com/bdtfs/gnat/internal/runner"
	repository "github.com/bdtfs/gnat/internal/storage/memory"
)

func TestNew(t *testing.T) {
	t.Parallel()

	repo := repository.New()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	collector := runner.NewCollector()
	r := runner.New(repo, logger, collector)

	svc := New(repo, r)

	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestService_CreateSetup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		url         string
		rps         int
		duration    time.Duration
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid setup",
			url:      "http://example.com",
			rps:      100,
			duration: time.Second,
			wantErr:  false,
		},
		{
			name:        "empty url",
			url:         "",
			rps:         100,
			duration:    time.Second,
			wantErr:     true,
			errContains: "url is required",
		},
		{
			name:        "zero rps",
			url:         "http://example.com",
			rps:         0,
			duration:    time.Second,
			wantErr:     true,
			errContains: "rps must be greater than 0",
		},
		{
			name:        "negative rps",
			url:         "http://example.com",
			rps:         -1,
			duration:    time.Second,
			wantErr:     true,
			errContains: "rps must be greater than 0",
		},
		{
			name:        "zero duration",
			url:         "http://example.com",
			rps:         100,
			duration:    0,
			wantErr:     true,
			errContains: "duration must be greater than 0",
		},
		{
			name:        "negative duration",
			url:         "http://example.com",
			rps:         100,
			duration:    -time.Second,
			wantErr:     true,
			errContains: "duration must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := newTestService()

			setup, err := svc.CreateSetup("test", "desc", "GET", tt.url, nil, nil, tt.rps, tt.duration)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if setup == nil {
				t.Error("expected non-nil setup")
				return
			}

			if setup.URL != tt.url {
				t.Errorf("expected URL %q, got %q", tt.url, setup.URL)
			}
			if setup.RPS != tt.rps {
				t.Errorf("expected RPS %d, got %d", tt.rps, setup.RPS)
			}
		})
	}
}

func TestService_CreateSetup_FullParams(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	setup, err := svc.CreateSetup(
		"Full Test",
		"Full description",
		"POST",
		"https://api.example.com/resource",
		[]byte(`{"key": "value"}`),
		map[string]string{"Content-Type": "application/json", "Authorization": "Bearer token"},
		200,
		2*time.Minute,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if setup.Name != "Full Test" {
		t.Errorf("expected Name 'Full Test', got %q", setup.Name)
	}
	if setup.Description != "Full description" {
		t.Errorf("expected Description 'Full description', got %q", setup.Description)
	}
	if setup.Method != "POST" {
		t.Errorf("expected Method 'POST', got %q", setup.Method)
	}
	if setup.RPS != 200 {
		t.Errorf("expected RPS 200, got %d", setup.RPS)
	}
	if setup.Duration != 2*time.Minute {
		t.Errorf("expected Duration 2m, got %v", setup.Duration)
	}
	if len(setup.Headers) != 2 {
		t.Errorf("expected 2 headers, got %d", len(setup.Headers))
	}
}

func TestService_GetSetup(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	created, _ := svc.CreateSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)

	t.Run("existing setup", func(t *testing.T) {
		t.Parallel()

		got, err := svc.GetSetup(created.ID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("expected ID %q, got %q", created.ID, got.ID)
		}
	})

	t.Run("non-existing setup", func(t *testing.T) {
		t.Parallel()

		_, err := svc.GetSetup("non-existing-id")
		if err == nil {
			t.Error("expected error for non-existing setup, got nil")
		}
	})
}

func TestService_ListSetups(t *testing.T) {
	t.Parallel()

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()

		svc := newTestService()
		setups := svc.ListSetups()

		if len(setups) != 0 {
			t.Errorf("expected 0 setups, got %d", len(setups))
		}
	})

	t.Run("multiple setups", func(t *testing.T) {
		t.Parallel()

		svc := newTestService()
		for i := 0; i < 5; i++ {
			svc.CreateSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
		}

		setups := svc.ListSetups()
		if len(setups) != 5 {
			t.Errorf("expected 5 setups, got %d", len(setups))
		}
	})
}

func TestService_UpdateSetup(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	created, _ := svc.CreateSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)

	t.Run("existing setup", func(t *testing.T) {
		created.Name = "updated name"
		originalUpdatedAt := created.UpdatedAt

		err := svc.UpdateSetup(created)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		got, _ := svc.GetSetup(created.ID)
		if got.Name != "updated name" {
			t.Errorf("expected updated name, got %q", got.Name)
		}
		if !got.UpdatedAt.After(originalUpdatedAt) {
			t.Error("expected UpdatedAt to be updated")
		}
	})

	t.Run("non-existing setup", func(t *testing.T) {
		t.Parallel()

		nonExisting := &models.Setup{ID: "non-existing"}
		err := svc.UpdateSetup(nonExisting)
		if err == nil {
			t.Error("expected error for non-existing setup, got nil")
		}
	})
}

func TestService_DeleteSetup(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	created, _ := svc.CreateSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)

	t.Run("existing setup", func(t *testing.T) {
		err := svc.DeleteSetup(created.ID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		_, err = svc.GetSetup(created.ID)
		if err == nil {
			t.Error("expected error getting deleted setup, got nil")
		}
	})

	t.Run("non-existing setup", func(t *testing.T) {
		t.Parallel()

		err := svc.DeleteSetup("non-existing-id")
		if err == nil {
			t.Error("expected error for non-existing setup, got nil")
		}
	})
}

func TestService_GetRun(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	// Create a run directly via repository for testing
	run := models.NewRun("setup-1")
	svc.repo.CreateRun(run)

	t.Run("existing run", func(t *testing.T) {
		t.Parallel()

		got, err := svc.GetRun(run.ID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got.ID != run.ID {
			t.Errorf("expected ID %q, got %q", run.ID, got.ID)
		}
	})

	t.Run("non-existing run", func(t *testing.T) {
		t.Parallel()

		_, err := svc.GetRun("non-existing-id")
		if err == nil {
			t.Error("expected error for non-existing run, got nil")
		}
	})
}

func TestService_ListRuns(t *testing.T) {
	t.Parallel()

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()

		svc := newTestService()
		runs := svc.ListRuns()

		if len(runs) != 0 {
			t.Errorf("expected 0 runs, got %d", len(runs))
		}
	})

	t.Run("multiple runs", func(t *testing.T) {
		t.Parallel()

		svc := newTestService()
		for i := 0; i < 5; i++ {
			run := models.NewRun("setup-1")
			svc.repo.CreateRun(run)
		}

		runs := svc.ListRuns()
		if len(runs) != 5 {
			t.Errorf("expected 5 runs, got %d", len(runs))
		}
	})
}

func TestService_ListRunsBySetup(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	// Create runs for different setups
	for i := 0; i < 3; i++ {
		run := models.NewRun("setup-1")
		svc.repo.CreateRun(run)
	}
	for i := 0; i < 2; i++ {
		run := models.NewRun("setup-2")
		svc.repo.CreateRun(run)
	}

	t.Run("setup with 3 runs", func(t *testing.T) {
		t.Parallel()

		runs := svc.ListRunsBySetup("setup-1")
		if len(runs) != 3 {
			t.Errorf("expected 3 runs, got %d", len(runs))
		}
	})

	t.Run("setup with 2 runs", func(t *testing.T) {
		t.Parallel()

		runs := svc.ListRunsBySetup("setup-2")
		if len(runs) != 2 {
			t.Errorf("expected 2 runs, got %d", len(runs))
		}
	})

	t.Run("setup with no runs", func(t *testing.T) {
		t.Parallel()

		runs := svc.ListRunsBySetup("non-existing")
		if len(runs) != 0 {
			t.Errorf("expected 0 runs, got %d", len(runs))
		}
	})
}

func TestService_StartRun_InvalidSetup(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	_, err := svc.StartRun(context.Background(), "non-existing-setup")
	if err == nil {
		t.Error("expected error for non-existing setup, got nil")
	}
}

func TestService_StartRun_InactiveSetup(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	// Create setup
	setup, _ := svc.CreateSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)

	// Mark it as inactive
	setup.Status = models.SetupStatusInactive
	svc.repo.UpdateSetup(setup)

	_, err := svc.StartRun(context.Background(), setup.ID)
	if err == nil {
		t.Error("expected error for inactive setup, got nil")
	}
}

func TestService_CancelRun_NotActive(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	err := svc.CancelRun("non-active-run")
	if err == nil {
		t.Error("expected error for non-active run, got nil")
	}
}

func TestService_GetActiveRuns_Empty(t *testing.T) {
	t.Parallel()

	svc := newTestService()

	runs := svc.GetActiveRuns()
	if len(runs) != 0 {
		t.Errorf("expected 0 active runs, got %d", len(runs))
	}
}

// Helper functions

func newTestService() *Service {
	repo := repository.New()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	collector := runner.NewCollector()
	r := runner.New(repo, logger, collector)
	return New(repo, r)
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
