package sqlite

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bdtfs/gnat/internal/models"
)

func newTestRepo(t *testing.T) *Repository {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	repo, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	t.Cleanup(func() {
		repo.Close()
	})

	return repo
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("valid path", func(t *testing.T) {
		t.Parallel()

		repo := newTestRepo(t)
		if repo == nil {
			t.Fatal("expected non-nil repository")
		}
		if repo.db == nil {
			t.Error("expected non-nil database")
		}
	})

	t.Run("in-memory database", func(t *testing.T) {
		t.Parallel()

		repo, err := New(":memory:")
		if err != nil {
			t.Fatalf("failed to create in-memory repository: %v", err)
		}
		defer repo.Close()

		if repo == nil {
			t.Fatal("expected non-nil repository")
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		t.Parallel()

		_, err := New("/nonexistent/deep/nested/path/test.db")
		if err == nil {
			t.Error("expected error for invalid path, got nil")
		}
	})
}

func TestClose(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	repo, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	err = repo.Close()
	if err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}

func TestTablesCreated(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	// Verify tables exist by querying sqlite_master.
	tables := []string{"setups", "runs", "stats"}
	for _, table := range tables {
		var name string
		err := repo.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Errorf("expected table %q to exist, got error: %v", table, err)
		}
	}
}

// --- Setup CRUD tests ---

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
		{
			name: "setup with body and headers",
			setup: models.NewSetup(
				"with-body",
				"description",
				"POST",
				"http://api.example.com/data",
				[]byte(`{"key":"value","nested":{"a":1}}`),
				map[string]string{
					"Content-Type":  "application/json",
					"Authorization": "Bearer token123",
					"X-Custom":      "custom-value",
				},
				200,
				30*time.Second,
			),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := newTestRepo(t)
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

	repo := newTestRepo(t)
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

	repo := newTestRepo(t)
	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	t.Run("existing setup", func(t *testing.T) {
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
		if got.Description != setup.Description {
			t.Errorf("expected Description %q, got %q", setup.Description, got.Description)
		}
		if got.Method != setup.Method {
			t.Errorf("expected Method %q, got %q", setup.Method, got.Method)
		}
		if got.URL != setup.URL {
			t.Errorf("expected URL %q, got %q", setup.URL, got.URL)
		}
		if got.RPS != setup.RPS {
			t.Errorf("expected RPS %d, got %d", setup.RPS, got.RPS)
		}
		if got.Duration != setup.Duration {
			t.Errorf("expected Duration %v, got %v", setup.Duration, got.Duration)
		}
		if got.Status != setup.Status {
			t.Errorf("expected Status %q, got %q", setup.Status, got.Status)
		}
	})

	t.Run("non-existing setup", func(t *testing.T) {
		_, err := repo.GetSetup("non-existing-id")
		if err == nil {
			t.Error("expected error for non-existing setup, got nil")
		}
	})
}

func TestRepository_GetSetup_WithHeadersAndBody(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	body := []byte(`{"key":"value"}`)
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token",
	}
	setup := models.NewSetup("full", "desc", "POST", "http://api.example.com", body, headers, 50, time.Minute)
	repo.CreateSetup(setup)

	got, err := repo.GetSetup(setup.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(got.Body) != string(body) {
		t.Errorf("expected Body %q, got %q", string(body), string(got.Body))
	}

	if len(got.Headers) != len(headers) {
		t.Errorf("expected %d headers, got %d", len(headers), len(got.Headers))
	}
	for k, v := range headers {
		if got.Headers[k] != v {
			t.Errorf("expected header %q=%q, got %q", k, v, got.Headers[k])
		}
	}
}

func TestRepository_ListSetups(t *testing.T) {
	t.Parallel()

	t.Run("empty repository", func(t *testing.T) {
		t.Parallel()

		repo := newTestRepo(t)
		setups := repo.ListSetups()

		if len(setups) != 0 {
			t.Errorf("expected 0 setups, got %d", len(setups))
		}
	})

	t.Run("multiple setups", func(t *testing.T) {
		t.Parallel()

		repo := newTestRepo(t)
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

	repo := newTestRepo(t)
	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	t.Run("existing setup", func(t *testing.T) {
		setup.Name = "updated name"
		setup.RPS = 500
		setup.UpdatedAt = time.Now()
		err := repo.UpdateSetup(setup)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		got, _ := repo.GetSetup(setup.ID)
		if got.Name != "updated name" {
			t.Errorf("expected updated name, got %q", got.Name)
		}
		if got.RPS != 500 {
			t.Errorf("expected RPS 500, got %d", got.RPS)
		}
	})

	t.Run("non-existing setup", func(t *testing.T) {
		nonExisting := &models.Setup{ID: "non-existing"}
		err := repo.UpdateSetup(nonExisting)
		if err == nil {
			t.Error("expected error for non-existing setup, got nil")
		}
	})
}

func TestRepository_DeleteSetup(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
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
		err := repo.DeleteSetup("non-existing-id")
		if err == nil {
			t.Error("expected error for non-existing setup, got nil")
		}
	})
}

// --- Run CRUD tests ---

func TestRepository_CreateRun(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	// Create a setup first (foreign key constraint).
	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	tests := []struct {
		name    string
		run     *models.Run
		wantErr bool
	}{
		{
			name:    "valid run",
			run:     models.NewRun(setup.ID),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	run := models.NewRun(setup.ID)

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

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	run := models.NewRun(setup.ID)
	repo.CreateRun(run)

	t.Run("existing run", func(t *testing.T) {
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
		if got.Status != run.Status {
			t.Errorf("expected Status %q, got %q", run.Status, got.Status)
		}
	})

	t.Run("non-existing run", func(t *testing.T) {
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

		repo := newTestRepo(t)
		runs := repo.ListRuns()

		if len(runs) != 0 {
			t.Errorf("expected 0 runs, got %d", len(runs))
		}
	})

	t.Run("multiple runs", func(t *testing.T) {
		t.Parallel()

		repo := newTestRepo(t)

		setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
		repo.CreateSetup(setup)

		for i := 0; i < 5; i++ {
			run := models.NewRun(setup.ID)
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

	repo := newTestRepo(t)

	setup1 := models.NewSetup("setup-1", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	setup2 := models.NewSetup("setup-2", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup1)
	repo.CreateSetup(setup2)

	// Create runs for setup-1.
	for i := 0; i < 3; i++ {
		run := models.NewRun(setup1.ID)
		repo.CreateRun(run)
	}

	// Create runs for setup-2.
	for i := 0; i < 2; i++ {
		run := models.NewRun(setup2.ID)
		repo.CreateRun(run)
	}

	t.Run("setup with runs", func(t *testing.T) {
		runs := repo.ListRunsBySetup(setup1.ID)
		if len(runs) != 3 {
			t.Errorf("expected 3 runs for setup-1, got %d", len(runs))
		}
	})

	t.Run("setup with fewer runs", func(t *testing.T) {
		runs := repo.ListRunsBySetup(setup2.ID)
		if len(runs) != 2 {
			t.Errorf("expected 2 runs for setup-2, got %d", len(runs))
		}
	})

	t.Run("setup with no runs", func(t *testing.T) {
		runs := repo.ListRunsBySetup("non-existing-setup")
		if len(runs) != 0 {
			t.Errorf("expected 0 runs, got %d", len(runs))
		}
	})
}

func TestRepository_UpdateRun(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	run := models.NewRun(setup.ID)
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

	t.Run("update with ended_at", func(t *testing.T) {
		run.Status = models.RunStatusCompleted
		run.EndedAt = time.Now()
		err := repo.UpdateRun(run)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		got, _ := repo.GetRun(run.ID)
		if got.Status != models.RunStatusCompleted {
			t.Errorf("expected status %q, got %q", models.RunStatusCompleted, got.Status)
		}
		if got.EndedAt.IsZero() {
			t.Error("expected non-zero EndedAt")
		}
	})

	t.Run("update with error", func(t *testing.T) {
		run.Status = models.RunStatusFailed
		run.Error = "connection timeout"
		err := repo.UpdateRun(run)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		got, _ := repo.GetRun(run.ID)
		if got.Error != "connection timeout" {
			t.Errorf("expected error %q, got %q", "connection timeout", got.Error)
		}
	})

	t.Run("non-existing run", func(t *testing.T) {
		nonExisting := &models.Run{ID: "non-existing"}
		err := repo.UpdateRun(nonExisting)
		if err == nil {
			t.Error("expected error for non-existing run, got nil")
		}
	})
}

func TestRepository_DeleteRun(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	run := models.NewRun(setup.ID)
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
		err := repo.DeleteRun("non-existing-id")
		if err == nil {
			t.Error("expected error for non-existing run, got nil")
		}
	})
}

// --- Stats tests ---

func TestRepository_SaveAndRetrieveStats(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	run := models.NewRun(setup.ID)
	repo.CreateRun(run)

	// Build stats with known values.
	stats := &models.Stats{
		TotalRequests:   1000,
		SuccessRequests: 950,
		FailedRequests:  50,
		TotalBytesRead:  1024000,
		StatusCodes:     make(map[int]*uint64),
		Latencies:       make([]time.Duration, 0),
		Errors:          []string{"timeout", "connection refused"},
	}

	// Add status codes.
	count200 := uint64(900)
	count500 := uint64(50)
	count301 := uint64(50)
	stats.StatusCodes[200] = &count200
	stats.StatusCodes[500] = &count500
	stats.StatusCodes[301] = &count301

	// Add some latencies for percentile calculation.
	for i := 0; i < 100; i++ {
		stats.Latencies = append(stats.Latencies, time.Duration(i+1)*time.Millisecond)
	}

	run.Stats = stats
	err := repo.UpdateRun(run)
	if err != nil {
		t.Fatalf("failed to update run with stats: %v", err)
	}

	// Retrieve the run and verify stats.
	got, err := repo.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}

	if got.Stats == nil {
		t.Fatal("expected non-nil stats")
	}

	if got.Stats.TotalRequests != 1000 {
		t.Errorf("expected TotalRequests 1000, got %d", got.Stats.TotalRequests)
	}
	if got.Stats.SuccessRequests != 950 {
		t.Errorf("expected SuccessRequests 950, got %d", got.Stats.SuccessRequests)
	}
	if got.Stats.FailedRequests != 50 {
		t.Errorf("expected FailedRequests 50, got %d", got.Stats.FailedRequests)
	}
	if got.Stats.TotalBytesRead != 1024000 {
		t.Errorf("expected TotalBytesRead 1024000, got %d", got.Stats.TotalBytesRead)
	}

	// Check status codes.
	if got.Stats.StatusCodes[200] == nil || *got.Stats.StatusCodes[200] != 900 {
		t.Error("expected status code 200 count of 900")
	}
	if got.Stats.StatusCodes[500] == nil || *got.Stats.StatusCodes[500] != 50 {
		t.Error("expected status code 500 count of 50")
	}
	if got.Stats.StatusCodes[301] == nil || *got.Stats.StatusCodes[301] != 50 {
		t.Error("expected status code 301 count of 50")
	}

	// Check errors.
	if len(got.Stats.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(got.Stats.Errors))
	}
}

func TestRepository_StatsPersistedWithRunUpdate(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	run := models.NewRun(setup.ID)
	repo.CreateRun(run)

	// First update: set stats.
	stats := &models.Stats{
		TotalRequests:   100,
		SuccessRequests: 90,
		FailedRequests:  10,
		StatusCodes:     make(map[int]*uint64),
		Latencies:       make([]time.Duration, 0),
		Errors:          make([]string, 0),
	}
	run.Stats = stats
	run.Status = models.RunStatusRunning
	repo.UpdateRun(run)

	// Second update: update stats.
	stats.TotalRequests = 200
	stats.SuccessRequests = 180
	stats.FailedRequests = 20
	run.Status = models.RunStatusCompleted
	run.EndedAt = time.Now()
	repo.UpdateRun(run)

	// Verify the final stats.
	got, err := repo.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}

	if got.Stats == nil {
		t.Fatal("expected non-nil stats after second update")
	}
	if got.Stats.TotalRequests != 200 {
		t.Errorf("expected TotalRequests 200 after update, got %d", got.Stats.TotalRequests)
	}
}

func TestRepository_RunWithoutStats(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	run := models.NewRun(setup.ID)
	repo.CreateRun(run)

	got, err := repo.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run: %v", err)
	}

	if got.Stats != nil {
		t.Error("expected nil stats for run without stats")
	}
}

func TestRepository_DeleteRunWithStats(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	run := models.NewRun(setup.ID)
	repo.CreateRun(run)

	// Add stats.
	run.Stats = &models.Stats{
		TotalRequests: 100,
		StatusCodes:   make(map[int]*uint64),
		Latencies:     make([]time.Duration, 0),
		Errors:        make([]string, 0),
	}
	repo.UpdateRun(run)

	// Delete the run.
	err := repo.DeleteRun(run.ID)
	if err != nil {
		t.Errorf("unexpected error deleting run with stats: %v", err)
	}

	// Verify it's gone.
	_, err = repo.GetRun(run.ID)
	if err == nil {
		t.Error("expected error getting deleted run")
	}
}

// --- Concurrent access tests ---

func TestRepository_ConcurrentSetupAccess(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	const numGoroutines = 50

	var wg sync.WaitGroup

	// Concurrent creates.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			setup := models.NewSetup(fmt.Sprintf("setup-%d", id), "", "GET", "http://example.com", nil, nil, 100, time.Second)
			repo.CreateSetup(setup)
		}(i)
	}

	wg.Wait()

	// Verify all created.
	setups := repo.ListSetups()
	if len(setups) != numGoroutines {
		t.Errorf("expected %d setups, got %d", numGoroutines, len(setups))
	}

	// Concurrent reads.
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

	repo := newTestRepo(t)
	const numGoroutines = 50

	// Create setups first.
	for i := 0; i < 10; i++ {
		setup := models.NewSetup(fmt.Sprintf("setup-%d", i), "", "GET", "http://example.com", nil, nil, 100, time.Second)
		setup.ID = fmt.Sprintf("setup-%d", i)
		repo.CreateSetup(setup)
	}

	var wg sync.WaitGroup

	// Concurrent creates.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			run := models.NewRun(fmt.Sprintf("setup-%d", id%10))
			repo.CreateRun(run)
		}(i)
	}

	wg.Wait()

	// Verify all created.
	runs := repo.ListRuns()
	if len(runs) != numGoroutines {
		t.Errorf("expected %d runs, got %d", numGoroutines, len(runs))
	}

	// Concurrent reads and filtered queries.
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

	repo := newTestRepo(t)
	const numGoroutines = 30

	var wg sync.WaitGroup

	// Create some initial setups for run creation.
	for i := 0; i < 5; i++ {
		setup := models.NewSetup(fmt.Sprintf("initial-%d", i), "", "GET", "http://example.com", nil, nil, 100, time.Second)
		setup.ID = fmt.Sprintf("initial-%d", i)
		repo.CreateSetup(setup)
	}

	// Mixed concurrent operations.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(4)

		// Create setup.
		go func(id int) {
			defer wg.Done()
			setup := models.NewSetup(fmt.Sprintf("setup-%d", id), "", "GET", "http://example.com", nil, nil, 100, time.Second)
			repo.CreateSetup(setup)
		}(i)

		// Create run.
		go func(id int) {
			defer wg.Done()
			run := models.NewRun(fmt.Sprintf("initial-%d", id%5))
			repo.CreateRun(run)
		}(i)

		// List setups.
		go func() {
			defer wg.Done()
			repo.ListSetups()
		}()

		// List runs.
		go func() {
			defer wg.Done()
			repo.ListRuns()
		}()
	}

	wg.Wait()

	// Verify no panic and data integrity.
	setups := repo.ListSetups()
	runs := repo.ListRuns()

	// We should have 5 initial + numGoroutines new setups.
	expectedSetups := 5 + numGoroutines
	if len(setups) != expectedSetups {
		t.Errorf("expected %d setups, got %d", expectedSetups, len(setups))
	}
	if len(runs) != numGoroutines {
		t.Errorf("expected %d runs, got %d", numGoroutines, len(runs))
	}
}

// --- Data persistence test ---

func TestRepository_DataPersistence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	// Create a repo and add data.
	repo1, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create first repository: %v", err)
	}

	setup := models.NewSetup("persistent", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo1.CreateSetup(setup)

	run := models.NewRun(setup.ID)
	repo1.CreateRun(run)

	repo1.Close()

	// Reopen and verify data persists.
	repo2, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create second repository: %v", err)
	}
	defer repo2.Close()

	got, err := repo2.GetSetup(setup.ID)
	if err != nil {
		t.Fatalf("failed to get setup after reopen: %v", err)
	}
	if got.Name != "persistent" {
		t.Errorf("expected Name 'persistent', got %q", got.Name)
	}

	gotRun, err := repo2.GetRun(run.ID)
	if err != nil {
		t.Fatalf("failed to get run after reopen: %v", err)
	}
	if gotRun.SetupID != setup.ID {
		t.Errorf("expected SetupID %q, got %q", setup.ID, gotRun.SetupID)
	}
}

// --- Interface compliance test ---

func TestRepository_ImplementsStorageInterface(t *testing.T) {
	t.Parallel()

	// This test verifies at compile time that *Repository implements
	// the storage.Repository interface. It will fail to compile if
	// the interface is not satisfied.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "interface.db")

	repo, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer repo.Close()

	// Verify that the file was actually created.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected database file to exist")
	}
}

// --- Edge cases ---

func TestRepository_SetupStatusRoundtrip(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	// Update status to inactive.
	setup.Status = models.SetupStatusInactive
	setup.UpdatedAt = time.Now()
	repo.UpdateSetup(setup)

	got, _ := repo.GetSetup(setup.ID)
	if got.Status != models.SetupStatusInactive {
		t.Errorf("expected status %q, got %q", models.SetupStatusInactive, got.Status)
	}
}

func TestRepository_RunStatusTransitions(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	run := models.NewRun(setup.ID)
	repo.CreateRun(run)

	// pending -> running.
	run.Status = models.RunStatusRunning
	repo.UpdateRun(run)
	got, _ := repo.GetRun(run.ID)
	if got.Status != models.RunStatusRunning {
		t.Errorf("expected running, got %q", got.Status)
	}

	// running -> completed.
	run.Status = models.RunStatusCompleted
	run.EndedAt = time.Now()
	repo.UpdateRun(run)
	got, _ = repo.GetRun(run.ID)
	if got.Status != models.RunStatusCompleted {
		t.Errorf("expected completed, got %q", got.Status)
	}
}

func TestRepository_RunStatusCancelled(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	run := models.NewRun(setup.ID)
	repo.CreateRun(run)

	run.Status = models.RunStatusCancelled
	run.EndedAt = time.Now()
	repo.UpdateRun(run)

	got, _ := repo.GetRun(run.ID)
	if got.Status != models.RunStatusCancelled {
		t.Errorf("expected cancelled, got %q", got.Status)
	}
}

func TestRepository_RunStatusFailed(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	run := models.NewRun(setup.ID)
	repo.CreateRun(run)

	run.Status = models.RunStatusFailed
	run.Error = "something went wrong"
	run.EndedAt = time.Now()
	repo.UpdateRun(run)

	got, _ := repo.GetRun(run.ID)
	if got.Status != models.RunStatusFailed {
		t.Errorf("expected failed, got %q", got.Status)
	}
	if got.Error != "something went wrong" {
		t.Errorf("expected error message, got %q", got.Error)
	}
}

func TestRepository_EmptyStats(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)

	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	repo.CreateSetup(setup)

	run := models.NewRun(setup.ID)
	repo.CreateRun(run)

	// Save empty stats (as if a run just started).
	run.Stats = &models.Stats{
		TotalRequests:   0,
		SuccessRequests: 0,
		FailedRequests:  0,
		StatusCodes:     make(map[int]*uint64),
		Latencies:       make([]time.Duration, 0),
		Errors:          make([]string, 0),
	}
	repo.UpdateRun(run)

	got, _ := repo.GetRun(run.ID)
	if got.Stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if got.Stats.TotalRequests != 0 {
		t.Errorf("expected TotalRequests 0, got %d", got.Stats.TotalRequests)
	}
}

func TestRepository_LargeNumberOfSetups(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	const count = 200

	for i := 0; i < count; i++ {
		setup := models.NewSetup(fmt.Sprintf("setup-%d", i), "", "GET", "http://example.com", nil, nil, 100, time.Second)
		if err := repo.CreateSetup(setup); err != nil {
			t.Fatalf("failed to create setup %d: %v", i, err)
		}
	}

	setups := repo.ListSetups()
	if len(setups) != count {
		t.Errorf("expected %d setups, got %d", count, len(setups))
	}
}
