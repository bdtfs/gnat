package converters

import (
	"sync"
	"testing"
	"time"

	"github.com/bdtfs/gnat/internal/models"
)

func TestRunToDTO(t *testing.T) {
	t.Parallel()

	t.Run("run without stats", func(t *testing.T) {
		t.Parallel()

		run := &models.Run{
			ID:        "run-123",
			SetupID:   "setup-456",
			Status:    models.RunStatusPending,
			StartedAt: time.Now(),
		}

		d := RunToDTO(run)

		if d.ID != run.ID {
			t.Errorf("expected ID %q, got %q", run.ID, d.ID)
		}
		if d.SetupID != run.SetupID {
			t.Errorf("expected SetupID %q, got %q", run.SetupID, d.SetupID)
		}
		if d.Status != string(run.Status) {
			t.Errorf("expected Status %q, got %q", run.Status, d.Status)
		}
		if d.Stats != nil {
			t.Error("expected nil Stats")
		}
		if d.EndedAt != nil {
			t.Error("expected nil EndedAt")
		}
	})

	t.Run("run with stats and ended time", func(t *testing.T) {
		t.Parallel()

		startTime := time.Now().Add(-10 * time.Second)
		endTime := time.Now()
		run := &models.Run{
			ID:        "run-789",
			SetupID:   "setup-123",
			Status:    models.RunStatusCompleted,
			StartedAt: startTime,
			EndedAt:   endTime,
			Stats:     newTestStats(100, 95, 5),
		}

		d := RunToDTO(run)

		if d.ID != run.ID {
			t.Errorf("expected ID %q, got %q", run.ID, d.ID)
		}
		if d.Status != string(run.Status) {
			t.Errorf("expected Status %q, got %q", run.Status, d.Status)
		}
		if d.Stats == nil {
			t.Fatal("expected non-nil Stats")
		}
		if d.EndedAt == nil {
			t.Fatal("expected non-nil EndedAt")
		}
		if d.Stats.Total != 100 {
			t.Errorf("expected Total 100, got %d", d.Stats.Total)
		}
	})

	t.Run("run with error", func(t *testing.T) {
		t.Parallel()

		run := &models.Run{
			ID:        "run-error",
			SetupID:   "setup-1",
			Status:    models.RunStatusFailed,
			StartedAt: time.Now(),
			EndedAt:   time.Now(),
			Error:     "connection refused",
		}

		d := RunToDTO(run)

		if d.Error != run.Error {
			t.Errorf("expected Error %q, got %q", run.Error, d.Error)
		}
	})
}

func TestStatsToDTO(t *testing.T) {
	t.Parallel()

	t.Run("nil stats", func(t *testing.T) {
		t.Parallel()

		result := StatsToDTO(nil, time.Now(), time.Now())
		if result != nil {
			t.Error("expected nil result for nil stats")
		}
	})

	t.Run("empty stats", func(t *testing.T) {
		t.Parallel()

		stats := &models.Stats{
			StatusCodes: make(map[int]*uint64),
		}
		startTime := time.Now()
		endTime := startTime.Add(10 * time.Second)

		result := StatsToDTO(stats, startTime, endTime)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Total != 0 {
			t.Errorf("expected Total 0, got %d", result.Total)
		}
		if result.RPS != 0 {
			t.Errorf("expected RPS 0, got %f", result.RPS)
		}
		if result.SuccessRate != 0 {
			t.Errorf("expected SuccessRate 0, got %f", result.SuccessRate)
		}
	})

	t.Run("full stats", func(t *testing.T) {
		t.Parallel()

		stats := newTestStats(1000, 950, 50)
		startTime := time.Now()
		endTime := startTime.Add(10 * time.Second)

		result := StatsToDTO(stats, startTime, endTime)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Total != 1000 {
			t.Errorf("expected Total 1000, got %d", result.Total)
		}
		if result.Success != 950 {
			t.Errorf("expected Success 950, got %d", result.Success)
		}
		if result.Failed != 50 {
			t.Errorf("expected Failed 50, got %d", result.Failed)
		}
		if result.SuccessRate != 0.95 {
			t.Errorf("expected SuccessRate 0.95, got %f", result.SuccessRate)
		}
		// RPS should be 1000/10 = 100
		if result.RPS != 100 {
			t.Errorf("expected RPS 100, got %f", result.RPS)
		}
	})

	t.Run("status codes conversion", func(t *testing.T) {
		t.Parallel()

		stats := &models.Stats{
			StatusCodes: make(map[int]*uint64),
		}
		count200 := uint64(900)
		count500 := uint64(100)
		stats.StatusCodes[200] = &count200
		stats.StatusCodes[500] = &count500

		result := StatsToDTO(stats, time.Now(), time.Now().Add(time.Second))

		if result.StatusCodes[200] != 900 {
			t.Errorf("expected 200 status count 900, got %d", result.StatusCodes[200])
		}
		if result.StatusCodes[500] != 100 {
			t.Errorf("expected 500 status count 100, got %d", result.StatusCodes[500])
		}
	})

	t.Run("errors conversion", func(t *testing.T) {
		t.Parallel()

		stats := &models.Stats{
			StatusCodes: make(map[int]*uint64),
			Errors:      []string{"error 1", "error 2"},
		}

		result := StatsToDTO(stats, time.Now(), time.Now().Add(time.Second))

		if len(result.Errors) != 2 {
			t.Errorf("expected 2 errors, got %d", len(result.Errors))
		}
	})
}

func TestPercentile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sorted   []time.Duration
		p        float64
		expected float64
	}{
		{
			name:     "empty slice",
			sorted:   []time.Duration{},
			p:        0.50,
			expected: 0,
		},
		{
			name:     "single element p50",
			sorted:   []time.Duration{100 * time.Millisecond},
			p:        0.50,
			expected: 100,
		},
		{
			name:     "two elements p50",
			sorted:   []time.Duration{100 * time.Millisecond, 200 * time.Millisecond},
			p:        0.50,
			expected: 100,
		},
		{
			name:     "ten elements p90",
			sorted:   makeLatencies(10, 10*time.Millisecond),
			p:        0.90,
			expected: 80, // index 8 (9 * 0.90 = 8.1 -> 8), value at index 8 is 80ms
		},
		{
			name:     "ten elements p99",
			sorted:   makeLatencies(10, 10*time.Millisecond),
			p:        0.99,
			expected: 80, // index 8 (9 * 0.99 = 8.91 -> 8), value at index 8 is 80ms
		},
		{
			name:     "100 elements p50",
			sorted:   makeLatencies(100, time.Millisecond),
			p:        0.50,
			expected: 49, // index 49
		},
		{
			name:     "100 elements p95",
			sorted:   makeLatencies(100, time.Millisecond),
			p:        0.95,
			expected: 94, // index 94
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := percentile(tt.sorted, tt.p)
			if result != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestStatsToDTO_Latencies(t *testing.T) {
	t.Parallel()

	stats := &models.Stats{
		TotalRequests:   100,
		SuccessRequests: 100,
		StatusCodes:     make(map[int]*uint64),
	}

	// Add latencies from 10ms to 1000ms (100 values, step of 10ms)
	for i := 1; i <= 100; i++ {
		stats.Latencies = append(stats.Latencies, time.Duration(i*10)*time.Millisecond)
	}

	startTime := time.Now()
	endTime := startTime.Add(10 * time.Second)
	result := StatsToDTO(stats, startTime, endTime)

	// Min should be 10ms
	if result.MinLatency != 10 {
		t.Errorf("expected MinLatency 10, got %f", result.MinLatency)
	}

	// Max should be 1000ms
	if result.MaxLatency != 1000 {
		t.Errorf("expected MaxLatency 1000, got %f", result.MaxLatency)
	}

	// Average should be (10+20+...+1000)/100 = 505ms
	expectedAvg := 505.0
	if result.AvgLatency != expectedAvg {
		t.Errorf("expected AvgLatency %f, got %f", expectedAvg, result.AvgLatency)
	}

	// P50 should be around 500ms (index 49 -> 500ms)
	if result.P50Latency != 500 {
		t.Errorf("expected P50Latency 500, got %f", result.P50Latency)
	}

	// P90 should be around 900ms (index 89 -> 900ms)
	if result.P90Latency != 900 {
		t.Errorf("expected P90Latency 900, got %f", result.P90Latency)
	}

	// P95 should be around 950ms (index 94 -> 950ms)
	if result.P95Latency != 950 {
		t.Errorf("expected P95Latency 950, got %f", result.P95Latency)
	}

	// P99 should be around 990ms (index 98 -> 990ms)
	if result.P99Latency != 990 {
		t.Errorf("expected P99Latency 990, got %f", result.P99Latency)
	}
}

// Helper functions

func newTestStats(total, success, failed uint64) *models.Stats {
	stats := &models.Stats{
		TotalRequests:   total,
		SuccessRequests: success,
		FailedRequests:  failed,
		StatusCodes:     make(map[int]*uint64),
	}

	// Add some latencies
	for i := 0; i < int(total); i++ {
		stats.Latencies = append(stats.Latencies, time.Duration(i)*time.Millisecond)
	}

	return stats
}

func makeLatencies(count int, step time.Duration) []time.Duration {
	result := make([]time.Duration, count)
	for i := 0; i < count; i++ {
		result[i] = time.Duration(i) * step
	}
	return result
}

func TestStatsToDTO_ThreadSafety(t *testing.T) {
	t.Parallel()

	stats := &models.Stats{
		TotalRequests:   1000,
		SuccessRequests: 950,
		FailedRequests:  50,
		StatusCodes:     make(map[int]*uint64),
		Latencies:       make([]time.Duration, 100),
		Errors:          []string{"error1", "error2"},
	}

	count200 := uint64(950)
	count500 := uint64(50)
	stats.StatusCodes[200] = &count200
	stats.StatusCodes[500] = &count500

	startTime := time.Now()
	endTime := startTime.Add(10 * time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := StatsToDTO(stats, startTime, endTime)
			if result == nil {
				t.Error("expected non-nil result")
			}
		}()
	}

	wg.Wait()
}
