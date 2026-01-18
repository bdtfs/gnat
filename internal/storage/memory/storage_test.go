package memory

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bdtfs/gnat/internal/models"
)

func TestNew(t *testing.T) {
	t.Parallel()

	repo := New()

	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
	if repo.setups == nil {
		t.Error("expected non-nil setups map")
	}
	if repo.runs == nil {
		t.Error("expected non-nil runs map")
	}
}

// Setup CRUD tests (T029)

func TestRepository_CreateSetup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   *models.Setup
		wantErr bool
	}{
		{
			name:    "valid setup",
			setup:   models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second),
			wantErr: false,
		},
		{
			name:    "setup with all fields",
			setup:   models.NewSetup("full", "full desc", "POST", "http://api.example.com", []byte(`{}`), map[string]string{"Auth": "token"}, 50, time.Minute),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := New()
			err := repo.CreateSetup(tt.setup)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRepository_CreateSetup_Duplicate(t *testing.T) {
	t.Parallel()

	repo := New()
	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)

	err := repo.CreateSetup(setup)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	err = repo.CreateSetup(setup)
	if err == nil {
		t.Error("expected error for duplicate setup, got nil")
	}
}

func TestRepository_GetSetup(t *testing.T) {
	t.Parallel()

	repo := New()
	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	t.Run("existing setup", func(t *testing.T) {
		t.Parallel()

		got, err := repo.GetSetup(setup.ID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got.ID != setup.ID {
			t.Errorf("expected ID %q, got %q", setup.ID, got.ID)
		}
		if got.Name != setup.Name {
			t.Errorf("expected Name %q, got %q", setup.Name, got.Name)
		}
	})

	t.Run("non-existing setup", func(t *testing.T) {
		t.Parallel()

		_, err := repo.GetSetup("non-existing-id")
		if err == nil {
			t.Error("expected error for non-existing setup, got nil")
		}
	})
}

func TestRepository_ListSetups(t *testing.T) {
	t.Parallel()

	t.Run("empty repository", func(t *testing.T) {
		t.Parallel()

		repo := New()
		setups := repo.ListSetups()

		if len(setups) != 0 {
			t.Errorf("expected 0 setups, got %d", len(setups))
		}
	})

	t.Run("multiple setups", func(t *testing.T) {
		t.Parallel()

		repo := New()
		for i := 0; i < 5; i++ {
			setup := models.NewSetup(fmt.Sprintf("test-%d", i), "", "GET", "http://example.com", nil, nil, 100, time.Second)
			repo.CreateSetup(setup)
		}

		setups := repo.ListSetups()
		if len(setups) != 5 {
			t.Errorf("expected 5 setups, got %d", len(setups))
		}
	})
}

func TestRepository_UpdateSetup(t *testing.T) {
	t.Parallel()

	repo := New()
	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	t.Run("existing setup", func(t *testing.T) {
		setup.Name = "updated name"
		err := repo.UpdateSetup(setup)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		got, _ := repo.GetSetup(setup.ID)
		if got.Name != "updated name" {
			t.Errorf("expected updated name, got %q", got.Name)
		}
	})

	t.Run("non-existing setup", func(t *testing.T) {
		t.Parallel()

		nonExisting := &models.Setup{ID: "non-existing"}
		err := repo.UpdateSetup(nonExisting)
		if err == nil {
			t.Error("expected error for non-existing setup, got nil")
		}
	})
}

func TestRepository_DeleteSetup(t *testing.T) {
	t.Parallel()

	repo := New()
	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	t.Run("existing setup", func(t *testing.T) {
		err := repo.DeleteSetup(setup.ID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		_, err = repo.GetSetup(setup.ID)
		if err == nil {
			t.Error("expected error getting deleted setup, got nil")
		}
	})

	t.Run("non-existing setup", func(t *testing.T) {
		t.Parallel()

		err := repo.DeleteSetup("non-existing-id")
		if err == nil {
			t.Error("expected error for non-existing setup, got nil")
		}
	})
}

// Run CRUD tests (T030)

func TestRepository_CreateRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		run     *models.Run
		wantErr bool
	}{
		{
			name:    "valid run",
			run:     models.NewRun("setup-1"),
			wantErr: false,
		},
		{
			name:    "run with empty setup ID",
			run:     models.NewRun(""),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := New()
			err := repo.CreateRun(tt.run)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRepository_CreateRun_Duplicate(t *testing.T) {
	t.Parallel()

	repo := New()
	run := models.NewRun("setup-1")

	err := repo.CreateRun(run)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	err = repo.CreateRun(run)
	if err == nil {
		t.Error("expected error for duplicate run, got nil")
	}
}

func TestRepository_GetRun(t *testing.T) {
	t.Parallel()

	repo := New()
	run := models.NewRun("setup-1")
	repo.CreateRun(run)

	t.Run("existing run", func(t *testing.T) {
		t.Parallel()

		got, err := repo.GetRun(run.ID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got.ID != run.ID {
			t.Errorf("expected ID %q, got %q", run.ID, got.ID)
		}
		if got.SetupID != run.SetupID {
			t.Errorf("expected SetupID %q, got %q", run.SetupID, got.SetupID)
		}
	})

	t.Run("non-existing run", func(t *testing.T) {
		t.Parallel()

		_, err := repo.GetRun("non-existing-id")
		if err == nil {
			t.Error("expected error for non-existing run, got nil")
		}
	})
}

func TestRepository_ListRuns(t *testing.T) {
	t.Parallel()

	t.Run("empty repository", func(t *testing.T) {
		t.Parallel()

		repo := New()
		runs := repo.ListRuns()

		if len(runs) != 0 {
			t.Errorf("expected 0 runs, got %d", len(runs))
		}
	})

	t.Run("multiple runs", func(t *testing.T) {
		t.Parallel()

		repo := New()
		for i := 0; i < 5; i++ {
			run := models.NewRun(fmt.Sprintf("setup-%d", i))
			repo.CreateRun(run)
		}

		runs := repo.ListRuns()
		if len(runs) != 5 {
			t.Errorf("expected 5 runs, got %d", len(runs))
		}
	})
}

func TestRepository_ListRunsBySetup(t *testing.T) {
	t.Parallel()

	repo := New()

	// Create runs for setup-1
	for i := 0; i < 3; i++ {
		run := models.NewRun("setup-1")
		repo.CreateRun(run)
	}

	// Create runs for setup-2
	for i := 0; i < 2; i++ {
		run := models.NewRun("setup-2")
		repo.CreateRun(run)
	}

	t.Run("setup with runs", func(t *testing.T) {
		t.Parallel()

		runs := repo.ListRunsBySetup("setup-1")
		if len(runs) != 3 {
			t.Errorf("expected 3 runs for setup-1, got %d", len(runs))
		}
	})

	t.Run("setup with fewer runs", func(t *testing.T) {
		t.Parallel()

		runs := repo.ListRunsBySetup("setup-2")
		if len(runs) != 2 {
			t.Errorf("expected 2 runs for setup-2, got %d", len(runs))
		}
	})

	t.Run("setup with no runs", func(t *testing.T) {
		t.Parallel()

		runs := repo.ListRunsBySetup("non-existing-setup")
		if len(runs) != 0 {
			t.Errorf("expected 0 runs, got %d", len(runs))
		}
	})
}

func TestRepository_UpdateRun(t *testing.T) {
	t.Parallel()

	repo := New()
	run := models.NewRun("setup-1")
	repo.CreateRun(run)

	t.Run("existing run", func(t *testing.T) {
		run.Status = models.RunStatusRunning
		err := repo.UpdateRun(run)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		got, _ := repo.GetRun(run.ID)
		if got.Status != models.RunStatusRunning {
			t.Errorf("expected status %q, got %q", models.RunStatusRunning, got.Status)
		}
	})

	t.Run("non-existing run", func(t *testing.T) {
		t.Parallel()

		nonExisting := &models.Run{ID: "non-existing"}
		err := repo.UpdateRun(nonExisting)
		if err == nil {
			t.Error("expected error for non-existing run, got nil")
		}
	})
}

func TestRepository_DeleteRun(t *testing.T) {
	t.Parallel()

	repo := New()
	run := models.NewRun("setup-1")
	repo.CreateRun(run)

	t.Run("existing run", func(t *testing.T) {
		err := repo.DeleteRun(run.ID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		_, err = repo.GetRun(run.ID)
		if err == nil {
			t.Error("expected error getting deleted run, got nil")
		}
	})

	t.Run("non-existing run", func(t *testing.T) {
		t.Parallel()

		err := repo.DeleteRun("non-existing-id")
		if err == nil {
			t.Error("expected error for non-existing run, got nil")
		}
	})
}

// Concurrent access tests (T031)

func TestRepository_ConcurrentSetupAccess(t *testing.T) {
	t.Parallel()

	repo := New()
	const numGoroutines = 100

	var wg sync.WaitGroup

	// Concurrent creates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			setup := models.NewSetup(fmt.Sprintf("setup-%d", id), "", "GET", "http://example.com", nil, nil, 100, time.Second)
			repo.CreateSetup(setup)
		}(i)
	}

	wg.Wait()

	// Verify all created
	setups := repo.ListSetups()
	if len(setups) != numGoroutines {
		t.Errorf("expected %d setups, got %d", numGoroutines, len(setups))
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repo.ListSetups()
		}()
	}

	wg.Wait()
}

func TestRepository_ConcurrentRunAccess(t *testing.T) {
	t.Parallel()

	repo := New()
	const numGoroutines = 100

	var wg sync.WaitGroup

	// Concurrent creates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			run := models.NewRun(fmt.Sprintf("setup-%d", id%10))
			repo.CreateRun(run)
		}(i)
	}

	wg.Wait()

	// Verify all created
	runs := repo.ListRuns()
	if len(runs) != numGoroutines {
		t.Errorf("expected %d runs, got %d", numGoroutines, len(runs))
	}

	// Concurrent reads and writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			repo.ListRuns()
		}()
		go func(setupID string) {
			defer wg.Done()
			repo.ListRunsBySetup(setupID)
		}(fmt.Sprintf("setup-%d", i%10))
	}

	wg.Wait()
}

func TestRepository_ConcurrentMixedAccess(t *testing.T) {
	t.Parallel()

	repo := New()
	const numGoroutines = 50

	var wg sync.WaitGroup

	// Mixed concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(4)

		// Create setup
		go func(id int) {
			defer wg.Done()
			setup := models.NewSetup(fmt.Sprintf("setup-%d", id), "", "GET", "http://example.com", nil, nil, 100, time.Second)
			repo.CreateSetup(setup)
		}(i)

		// Create run
		go func(id int) {
			defer wg.Done()
			run := models.NewRun(fmt.Sprintf("setup-%d", id%10))
			repo.CreateRun(run)
		}(i)

		// List setups
		go func() {
			defer wg.Done()
			repo.ListSetups()
		}()

		// List runs
		go func() {
			defer wg.Done()
			repo.ListRuns()
		}()
	}

	wg.Wait()

	// Verify no panic and data integrity
	setups := repo.ListSetups()
	runs := repo.ListRuns()

	if len(setups) != numGoroutines {
		t.Errorf("expected %d setups, got %d", numGoroutines, len(setups))
	}
	if len(runs) != numGoroutines {
		t.Errorf("expected %d runs, got %d", numGoroutines, len(runs))
	}
}
