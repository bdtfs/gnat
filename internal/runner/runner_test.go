package runner

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bdtfs/gnat/internal/models"
	repository "github.com/bdtfs/gnat/internal/storage/memory"
)

func TestNew(t *testing.T) {
	t.Parallel()

	repo := repository.New()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	collector := NewCollector()

	runner := New(repo, logger, collector)

	if runner == nil {
		t.Fatal("expected non-nil runner")
	}
	if runner.repo != repo {
		t.Error("expected repo to be set")
	}
	if runner.logger != logger {
		t.Error("expected logger to be set")
	}
	if runner.collector != collector {
		t.Error("expected collector to be set")
	}
	if runner.activeRuns == nil {
		t.Error("expected activeRuns map to be initialized")
	}
}

func TestRunner_StartRun_SetupNotFound(t *testing.T) {
	t.Parallel()

	runner := newTestRunner()

	_, err := runner.StartRun(context.Background(), "non-existing-setup")
	if err == nil {
		t.Error("expected error for non-existing setup, got nil")
	}
}

func TestRunner_StartRun_InactiveSetup(t *testing.T) {
	t.Parallel()

	runner := newTestRunner()

	// Create an inactive setup
	setup := models.NewSetup("test", "desc", "GET", "http://example.com", nil, nil, 100, time.Second)
	setup.Status = models.SetupStatusInactive
	runner.repo.CreateSetup(setup)

	_, err := runner.StartRun(context.Background(), setup.ID)
	if err == nil {
		t.Error("expected error for inactive setup, got nil")
	}
}

func TestRunner_CancelRun_NotActive(t *testing.T) {
	t.Parallel()

	runner := newTestRunner()

	err := runner.CancelRun("non-active-run")
	if err == nil {
		t.Error("expected error for non-active run, got nil")
	}
}

func TestRunner_GetActiveRuns_Empty(t *testing.T) {
	t.Parallel()

	runner := newTestRunner()

	runs := runner.GetActiveRuns()
	if len(runs) != 0 {
		t.Errorf("expected 0 active runs, got %d", len(runs))
	}
}

func TestRunner_GetActiveRuns_WithRuns(t *testing.T) {
	t.Parallel()

	runner := newTestRunner()

	// Manually add some active runs for testing
	runner.activeRunsMu.Lock()
	runner.activeRuns["run-1"] = func() {}
	runner.activeRuns["run-2"] = func() {}
	runner.activeRunsMu.Unlock()

	runs := runner.GetActiveRuns()
	if len(runs) != 2 {
		t.Errorf("expected 2 active runs, got %d", len(runs))
	}
}

func TestRunner_CancelRun_Active(t *testing.T) {
	t.Parallel()

	runner := newTestRunner()

	cancelled := false
	cancelFunc := func() {
		cancelled = true
	}

	// Add an active run
	runner.activeRunsMu.Lock()
	runner.activeRuns["run-1"] = cancelFunc
	runner.activeRunsMu.Unlock()

	err := runner.CancelRun("run-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !cancelled {
		t.Error("expected cancel function to be called")
	}
}

// Collector tests

func TestNewCollector(t *testing.T) {
	t.Parallel()

	collector := NewCollector()

	if collector == nil {
		t.Fatal("expected non-nil collector")
	}
	if collector.runs == nil {
		t.Error("expected runs map to be initialized")
	}
}

func TestCollector_StartRunStatsProcessing(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	run := models.NewRun("setup-1")

	ch := collector.StartRunStatsProcessing(run)

	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
	if run.Stats == nil {
		t.Error("expected run.Stats to be set")
	}

	// Verify stats are tracked
	stats := collector.GetStats(run.ID)
	if stats == nil {
		t.Error("expected stats to be tracked")
	}

	close(ch)
}

func TestCollector_ProcessOneResult_Success(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	stats := NewStats()

	result := &Result{
		StatusCode: 200,
		Latency:    100 * time.Millisecond,
		BytesRead:  1024,
		Error:      nil,
	}

	collector.ProcessOneResult(stats, result)

	if stats.TotalRequests != 1 {
		t.Errorf("expected TotalRequests 1, got %d", stats.TotalRequests)
	}
	if stats.SuccessRequests != 1 {
		t.Errorf("expected SuccessRequests 1, got %d", stats.SuccessRequests)
	}
	if stats.FailedRequests != 0 {
		t.Errorf("expected FailedRequests 0, got %d", stats.FailedRequests)
	}
	if stats.TotalBytesRead != 1024 {
		t.Errorf("expected TotalBytesRead 1024, got %d", stats.TotalBytesRead)
	}
	if len(stats.Latencies) != 1 {
		t.Errorf("expected 1 latency, got %d", len(stats.Latencies))
	}
}

func TestCollector_ProcessOneResult_Error(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	stats := NewStats()

	result := &Result{
		Error: &testError{msg: "connection refused"},
	}

	collector.ProcessOneResult(stats, result)

	if stats.TotalRequests != 1 {
		t.Errorf("expected TotalRequests 1, got %d", stats.TotalRequests)
	}
	if stats.SuccessRequests != 0 {
		t.Errorf("expected SuccessRequests 0, got %d", stats.SuccessRequests)
	}
	if stats.FailedRequests != 1 {
		t.Errorf("expected FailedRequests 1, got %d", stats.FailedRequests)
	}
	if len(stats.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(stats.Errors))
	}
}

func TestCollector_ProcessOneResult_ClientError(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	stats := NewStats()

	result := &Result{
		StatusCode: 400,
		Latency:    50 * time.Millisecond,
		BytesRead:  100,
	}

	collector.ProcessOneResult(stats, result)

	if stats.SuccessRequests != 0 {
		t.Errorf("expected SuccessRequests 0 for 400 status, got %d", stats.SuccessRequests)
	}
	if stats.FailedRequests != 1 {
		t.Errorf("expected FailedRequests 1 for 400 status, got %d", stats.FailedRequests)
	}
}

func TestCollector_ProcessOneResult_ServerError(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	stats := NewStats()

	result := &Result{
		StatusCode: 500,
		Latency:    50 * time.Millisecond,
		BytesRead:  100,
	}

	collector.ProcessOneResult(stats, result)

	if stats.SuccessRequests != 0 {
		t.Errorf("expected SuccessRequests 0 for 500 status, got %d", stats.SuccessRequests)
	}
	if stats.FailedRequests != 1 {
		t.Errorf("expected FailedRequests 1 for 500 status, got %d", stats.FailedRequests)
	}
}

func TestCollector_ProcessOneResult_RedirectSuccess(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	stats := NewStats()

	// 3xx redirects should count as success
	result := &Result{
		StatusCode: 301,
		Latency:    50 * time.Millisecond,
		BytesRead:  100,
	}

	collector.ProcessOneResult(stats, result)

	if stats.SuccessRequests != 1 {
		t.Errorf("expected SuccessRequests 1 for 301 status, got %d", stats.SuccessRequests)
	}
	if stats.FailedRequests != 0 {
		t.Errorf("expected FailedRequests 0 for 301 status, got %d", stats.FailedRequests)
	}
}

func TestCollector_ProcessOneResult_StatusCodes(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	stats := NewStats()

	// Process multiple results with different status codes
	for i := 0; i < 10; i++ {
		collector.ProcessOneResult(stats, &Result{StatusCode: 200, Latency: time.Millisecond})
	}
	for i := 0; i < 3; i++ {
		collector.ProcessOneResult(stats, &Result{StatusCode: 500, Latency: time.Millisecond})
	}

	if *stats.StatusCodes[200] != 10 {
		t.Errorf("expected 200 status count 10, got %d", *stats.StatusCodes[200])
	}
	if *stats.StatusCodes[500] != 3 {
		t.Errorf("expected 500 status count 3, got %d", *stats.StatusCodes[500])
	}
}

func TestCollector_GetStats_NonExisting(t *testing.T) {
	t.Parallel()

	collector := NewCollector()

	stats := collector.GetStats("non-existing")
	if stats != nil {
		t.Error("expected nil stats for non-existing run")
	}
}

func TestCollector_DeleteRun(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	run := models.NewRun("setup-1")

	ch := collector.StartRunStatsProcessing(run)
	close(ch)

	// Verify stats exist
	if collector.GetStats(run.ID) == nil {
		t.Error("expected stats to exist before delete")
	}

	collector.DeleteRun(run.ID)

	if collector.GetStats(run.ID) != nil {
		t.Error("expected stats to be deleted")
	}
}

func TestCollector_ConcurrentProcessing(t *testing.T) {
	t.Parallel()

	collector := NewCollector()
	stats := NewStats()

	const numGoroutines = 100
	done := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			result := &Result{
				StatusCode: 200,
				Latency:    time.Duration(id) * time.Millisecond,
				BytesRead:  int64(id * 100),
			}
			collector.ProcessOneResult(stats, result)
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	if stats.TotalRequests != numGoroutines {
		t.Errorf("expected TotalRequests %d, got %d", numGoroutines, stats.TotalRequests)
	}
	if stats.SuccessRequests != numGoroutines {
		t.Errorf("expected SuccessRequests %d, got %d", numGoroutines, stats.SuccessRequests)
	}
}

// Helper types and functions

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func newTestRunner() *Runner {
	repo := repository.New()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	collector := NewCollector()
	return New(repo, logger, collector)
}
