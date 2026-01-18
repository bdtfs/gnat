package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/bdtfs/gnat/internal/models"
	"github.com/bdtfs/gnat/internal/runner"
	"github.com/bdtfs/gnat/internal/server/dto"
	"github.com/bdtfs/gnat/internal/service"
	repository "github.com/bdtfs/gnat/internal/storage/memory"
)

func TestNew(t *testing.T) {
	t.Parallel()

	srv := newTestServer()

	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv.server == nil {
		t.Error("expected http.Server to be initialized")
	}
}

func TestServer_CreateSetup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid setup",
			body:       `{"name":"Test","method":"GET","url":"http://example.com","rps":100,"duration":"30s"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid json",
			body:       `{invalid}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid duration",
			body:       `{"name":"Test","method":"GET","url":"http://example.com","rps":100,"duration":"invalid"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing url",
			body:       `{"name":"Test","method":"GET","rps":100,"duration":"30s"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "zero rps",
			body:       `{"name":"Test","method":"GET","url":"http://example.com","rps":0,"duration":"30s"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative duration",
			body:       `{"name":"Test","method":"GET","url":"http://example.com","rps":100,"duration":"-30s"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newTestServer()
			req := httptest.NewRequest(http.MethodPost, "/api/setups", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			srv.handleCreateSetup(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestServer_CreateSetup_Response(t *testing.T) {
	t.Parallel()

	srv := newTestServer()
	body := `{"name":"Test Setup","description":"A test","method":"POST","url":"http://api.example.com","rps":200,"duration":"1m"}`

	req := httptest.NewRequest(http.MethodPost, "/api/setups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.handleCreateSetup(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var setup dto.Setup
	if err := json.NewDecoder(rec.Body).Decode(&setup); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if setup.ID == "" {
		t.Error("expected non-empty ID")
	}
	if setup.Name != "Test Setup" {
		t.Errorf("expected Name 'Test Setup', got %q", setup.Name)
	}
	if setup.RPS != 200 {
		t.Errorf("expected RPS 200, got %d", setup.RPS)
	}
	if setup.Duration != time.Minute {
		t.Errorf("expected Duration 1m, got %v", setup.Duration)
	}
}

func TestServer_ListSetups(t *testing.T) {
	t.Parallel()

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()

		srv := newTestServer()
		req := httptest.NewRequest(http.MethodGet, "/api/setups", nil)
		rec := httptest.NewRecorder()

		srv.handleListSetups(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var setups []*dto.Setup
		if err := json.NewDecoder(rec.Body).Decode(&setups); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(setups) != 0 {
			t.Errorf("expected 0 setups, got %d", len(setups))
		}
	})

	t.Run("with setups", func(t *testing.T) {
		t.Parallel()

		srv := newTestServer()

		// Create some setups
		for i := 0; i < 3; i++ {
			srv.service.CreateSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/setups", nil)
		rec := httptest.NewRecorder()

		srv.handleListSetups(rec, req)

		var setups []*dto.Setup
		json.NewDecoder(rec.Body).Decode(&setups)

		if len(setups) != 3 {
			t.Errorf("expected 3 setups, got %d", len(setups))
		}
	})
}

func TestServer_GetSetup(t *testing.T) {
	t.Parallel()

	srv := newTestServer()

	// Create a setup
	setup, _ := srv.service.CreateSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)

	t.Run("existing setup", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/setups/"+setup.ID, nil)
		req.SetPathValue("id", setup.ID)
		rec := httptest.NewRecorder()

		srv.handleGetSetup(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var got dto.Setup
		json.NewDecoder(rec.Body).Decode(&got)

		if got.ID != setup.ID {
			t.Errorf("expected ID %q, got %q", setup.ID, got.ID)
		}
	})

	t.Run("non-existing setup", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/setups/non-existing", nil)
		req.SetPathValue("id", "non-existing")
		rec := httptest.NewRecorder()

		srv.handleGetSetup(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestServer_DeleteSetup(t *testing.T) {
	t.Parallel()

	srv := newTestServer()

	// Create a setup
	setup, _ := srv.service.CreateSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)

	t.Run("existing setup", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/setups/"+setup.ID, nil)
		req.SetPathValue("id", setup.ID)
		rec := httptest.NewRecorder()

		srv.handleDeleteSetup(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected status %d, got %d", http.StatusNoContent, rec.Code)
		}

		// Verify it's deleted
		_, err := srv.service.GetSetup(setup.ID)
		if err == nil {
			t.Error("expected error getting deleted setup")
		}
	})

	t.Run("non-existing setup", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodDelete, "/api/setups/non-existing", nil)
		req.SetPathValue("id", "non-existing")
		rec := httptest.NewRecorder()

		srv.handleDeleteSetup(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestServer_StartRun(t *testing.T) {
	t.Parallel()

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()

		srv := newTestServer()
		req := httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString("{invalid}"))
		rec := httptest.NewRecorder()

		srv.handleStartRun(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("non-existing setup", func(t *testing.T) {
		t.Parallel()

		srv := newTestServer()
		body := `{"setup_id":"non-existing"}`
		req := httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		srv.handleStartRun(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("inactive setup", func(t *testing.T) {
		t.Parallel()

		srv := newTestServer()

		// Create setup and make it inactive
		setup, _ := srv.service.CreateSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
		setup.Status = models.SetupStatusInactive
		srv.service.UpdateSetup(setup)

		body := `{"setup_id":"` + setup.ID + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		srv.handleStartRun(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

func TestServer_ListRuns(t *testing.T) {
	t.Parallel()

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()

		srv := newTestServer()
		req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
		rec := httptest.NewRecorder()

		srv.handleListRuns(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var runs []*dto.Run
		json.NewDecoder(rec.Body).Decode(&runs)

		if len(runs) != 0 {
			t.Errorf("expected 0 runs, got %d", len(runs))
		}
	})

	t.Run("with runs", func(t *testing.T) {
		t.Parallel()

		srv := newTestServer()

		// Create runs directly via repository
		for i := 0; i < 3; i++ {
			run := models.NewRun("setup-1")
			createTestRun(srv, run)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
		rec := httptest.NewRecorder()

		srv.handleListRuns(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var runs []*dto.Run
		json.NewDecoder(rec.Body).Decode(&runs)

		if len(runs) != 3 {
			t.Errorf("expected 3 runs, got %d", len(runs))
		}
	})

	t.Run("filter by setup_id", func(t *testing.T) {
		t.Parallel()

		srv := newTestServer()

		req := httptest.NewRequest(http.MethodGet, "/api/runs?setup_id=setup-1", nil)
		rec := httptest.NewRecorder()

		srv.handleListRuns(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})
}

func TestServer_GetRun(t *testing.T) {
	t.Parallel()

	srv := newTestServer()

	// Create a run directly via repository through service
	run := models.NewRun("setup-1")
	// We need to access the underlying repo - use a helper
	createTestRun(srv, run)

	t.Run("existing run", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/runs/"+run.ID, nil)
		req.SetPathValue("id", run.ID)
		rec := httptest.NewRecorder()

		srv.handleGetRun(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("non-existing run", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/runs/non-existing", nil)
		req.SetPathValue("id", "non-existing")
		rec := httptest.NewRecorder()

		srv.handleGetRun(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestServer_CancelRun(t *testing.T) {
	t.Parallel()

	t.Run("non-active run", func(t *testing.T) {
		t.Parallel()

		srv := newTestServer()
		req := httptest.NewRequest(http.MethodPost, "/api/runs/non-existing/cancel", nil)
		req.SetPathValue("id", "non-existing")
		rec := httptest.NewRecorder()

		srv.handleCancelRun(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

func TestServer_GetRunStats(t *testing.T) {
	t.Parallel()

	srv := newTestServer()

	// Create a run with stats
	run := models.NewRun("setup-1")
	run.Stats = &models.Stats{
		TotalRequests:   100,
		SuccessRequests: 95,
		FailedRequests:  5,
		StatusCodes:     make(map[int]*uint64),
	}
	createTestRun(srv, run)

	t.Run("existing run", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/runs/"+run.ID+"/stats", nil)
		req.SetPathValue("id", run.ID)
		rec := httptest.NewRecorder()

		srv.handleGetRunStats(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("non-existing run", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/api/runs/non-existing/stats", nil)
		req.SetPathValue("id", "non-existing")
		rec := httptest.NewRecorder()

		srv.handleGetRunStats(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestRespondJSON(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	respondJSON(rec, http.StatusOK, data)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", rec.Header().Get("Content-Type"))
	}

	var result map[string]string
	json.NewDecoder(rec.Body).Decode(&result)
	if result["key"] != "value" {
		t.Errorf("expected key='value', got key=%q", result["key"])
	}
}

func TestRespondError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	respondError(rec, http.StatusBadRequest, "something went wrong")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var result map[string]string
	json.NewDecoder(rec.Body).Decode(&result)
	if result["error"] != "something went wrong" {
		t.Errorf("expected error='something went wrong', got error=%q", result["error"])
	}
}

// Helper functions

type testServerHelper struct {
	*Server
	repo *repository.Repository
}

func newTestServer() *testServerHelper {
	repo := repository.New()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	collector := runner.NewCollector()
	r := runner.New(repo, logger, collector)
	svc := service.New(repo, r)
	srv := New(":0", svc, logger)
	return &testServerHelper{
		Server: srv,
		repo:   repo,
	}
}

func createTestRun(srv *testServerHelper, run *models.Run) {
	srv.repo.CreateRun(run)
}
