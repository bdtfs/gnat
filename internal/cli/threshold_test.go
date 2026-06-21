package cli

import (
	"sync"
	"testing"
	"time"

	"github.com/bdtfs/gnat/internal/models"
)

// fp is a helper that returns a pointer to a float64.
func fp(v float64) *float64 {
	return &v
}

// makeStats creates a models.Stats with the given latencies and request counts.
// elapsed is the intended test duration used alongside the stats for RPS computation.
func makeStats(latenciesMs []int, success, failed uint64) *models.Stats {
	latencies := make([]time.Duration, len(latenciesMs))
	for i, ms := range latenciesMs {
		latencies[i] = time.Duration(ms) * time.Millisecond
	}
	return &models.Stats{
		TotalRequests:   success + failed,
		SuccessRequests: success,
		FailedRequests:  failed,
		Latencies:       latencies,
		LatenciesMu:     sync.Mutex{},
		StatusMu:        sync.RWMutex{},
		ErrorsMu:        sync.RWMutex{},
	}
}

// ---------------------------------------------------------------------------
// Individual threshold type tests
// ---------------------------------------------------------------------------

func TestEvaluate_P50Latency_Pass(t *testing.T) {
	// 10 latencies: 10,20,30,40,50,60,70,80,90,100
	// P50 = sorted[4] = 50ms
	stats := makeStats([]int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 10, 0)
	th := Thresholds{P50LatencyMs: fp(50)}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected overall pass, got fail")
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	r := result.Results[0]
	if r.Name != "p50_latency_ms" {
		t.Errorf("expected name p50_latency_ms, got %s", r.Name)
	}
	if r.Actual != 50 {
		t.Errorf("expected actual 50, got %f", r.Actual)
	}
	if !r.Passed {
		t.Errorf("expected threshold to pass")
	}
}

func TestEvaluate_P50Latency_Fail(t *testing.T) {
	stats := makeStats([]int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 10, 0)
	th := Thresholds{P50LatencyMs: fp(49)}

	result := Evaluate(th, stats, 10*time.Second)
	if result.Passed {
		t.Fatalf("expected overall fail, got pass")
	}
	if !result.Results[0].Passed == false {
		t.Errorf("expected threshold to fail")
	}
}

func TestEvaluate_P90Latency_Pass(t *testing.T) {
	stats := makeStats([]int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 10, 0)
	// P90 = sorted[8] = 90ms
	th := Thresholds{P90LatencyMs: fp(90)}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass")
	}
	if result.Results[0].Actual != 90 {
		t.Errorf("expected actual 90, got %f", result.Results[0].Actual)
	}
}

func TestEvaluate_P90Latency_Fail(t *testing.T) {
	stats := makeStats([]int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 10, 0)
	th := Thresholds{P90LatencyMs: fp(89)}

	result := Evaluate(th, stats, 10*time.Second)
	if result.Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_P95Latency_Pass(t *testing.T) {
	// 20 latencies: 1..20
	latencies := make([]int, 20)
	for i := range latencies {
		latencies[i] = (i + 1) * 10
	}
	stats := makeStats(latencies, 20, 0)
	// P95 = sorted[int(19*0.95)] = sorted[18] = 190ms
	th := Thresholds{P95LatencyMs: fp(190)}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass, actual=%f", result.Results[0].Actual)
	}
}

func TestEvaluate_P95Latency_Fail(t *testing.T) {
	latencies := make([]int, 20)
	for i := range latencies {
		latencies[i] = (i + 1) * 10
	}
	stats := makeStats(latencies, 20, 0)
	th := Thresholds{P95LatencyMs: fp(189)}

	result := Evaluate(th, stats, 10*time.Second)
	if result.Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_P99Latency_Pass(t *testing.T) {
	// 100 latencies: 1..100ms
	latencies := make([]int, 100)
	for i := range latencies {
		latencies[i] = i + 1
	}
	stats := makeStats(latencies, 100, 0)
	// P99 = sorted[int(99*0.99)] = sorted[98] = 99ms
	th := Thresholds{P99LatencyMs: fp(99)}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass, actual=%f", result.Results[0].Actual)
	}
}

func TestEvaluate_P99Latency_Fail(t *testing.T) {
	latencies := make([]int, 100)
	for i := range latencies {
		latencies[i] = i + 1
	}
	stats := makeStats(latencies, 100, 0)
	th := Thresholds{P99LatencyMs: fp(98)}

	result := Evaluate(th, stats, 10*time.Second)
	if result.Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_AvgLatency_Pass(t *testing.T) {
	// Latencies: 10, 20, 30 -> avg = 20ms
	stats := makeStats([]int{10, 20, 30}, 3, 0)
	th := Thresholds{AvgLatencyMs: fp(20)}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass, actual=%f", result.Results[0].Actual)
	}
}

func TestEvaluate_AvgLatency_Fail(t *testing.T) {
	stats := makeStats([]int{10, 20, 30}, 3, 0)
	th := Thresholds{AvgLatencyMs: fp(19)}

	result := Evaluate(th, stats, 10*time.Second)
	if result.Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_MaxLatency_Pass(t *testing.T) {
	stats := makeStats([]int{10, 20, 30, 40, 50}, 5, 0)
	th := Thresholds{MaxLatencyMs: fp(50)}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass")
	}
	if result.Results[0].Actual != 50 {
		t.Errorf("expected actual 50, got %f", result.Results[0].Actual)
	}
}

func TestEvaluate_MaxLatency_Fail(t *testing.T) {
	stats := makeStats([]int{10, 20, 30, 40, 50}, 5, 0)
	th := Thresholds{MaxLatencyMs: fp(49)}

	result := Evaluate(th, stats, 10*time.Second)
	if result.Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_ErrorRate_Pass(t *testing.T) {
	// 90 success + 10 failed = 10% error rate
	stats := makeStats([]int{10}, 90, 10)
	th := Thresholds{ErrorRate: fp(0.10)}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass, actual=%f", result.Results[0].Actual)
	}
	if result.Results[0].Actual != 0.10 {
		t.Errorf("expected actual 0.10, got %f", result.Results[0].Actual)
	}
}

func TestEvaluate_ErrorRate_Fail(t *testing.T) {
	stats := makeStats([]int{10}, 90, 10)
	th := Thresholds{ErrorRate: fp(0.09)}

	result := Evaluate(th, stats, 10*time.Second)
	if result.Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_ErrorRate_ZeroErrors(t *testing.T) {
	stats := makeStats([]int{10}, 100, 0)
	th := Thresholds{ErrorRate: fp(0.01)}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass with zero errors")
	}
	if result.Results[0].Actual != 0.0 {
		t.Errorf("expected actual 0.0, got %f", result.Results[0].Actual)
	}
}

func TestEvaluate_MinRPS_Pass(t *testing.T) {
	// 100 requests in 10 seconds = 10 RPS
	stats := makeStats([]int{10}, 100, 0)
	th := Thresholds{MinRPS: fp(10)}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass, actual=%f", result.Results[0].Actual)
	}
}

func TestEvaluate_MinRPS_Fail(t *testing.T) {
	stats := makeStats([]int{10}, 100, 0)
	th := Thresholds{MinRPS: fp(11)}

	result := Evaluate(th, stats, 10*time.Second)
	if result.Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_MinSuccessRate_Pass(t *testing.T) {
	// 95/100 = 0.95 success rate
	stats := makeStats([]int{10}, 95, 5)
	th := Thresholds{MinSuccessRate: fp(0.95)}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass, actual=%f", result.Results[0].Actual)
	}
}

func TestEvaluate_MinSuccessRate_Fail(t *testing.T) {
	stats := makeStats([]int{10}, 95, 5)
	th := Thresholds{MinSuccessRate: fp(0.96)}

	result := Evaluate(th, stats, 10*time.Second)
	if result.Passed {
		t.Fatalf("expected fail")
	}
}

// ---------------------------------------------------------------------------
// Multiple thresholds combined
// ---------------------------------------------------------------------------

func TestEvaluate_AllThresholds_AllPass(t *testing.T) {
	// 100 latencies: 1..100ms, 95 success, 5 failed, 10s elapsed
	latencies := make([]int, 100)
	for i := range latencies {
		latencies[i] = i + 1
	}
	stats := makeStats(latencies, 95, 5)
	elapsed := 10 * time.Second

	th := Thresholds{
		P50LatencyMs:   fp(100),
		P90LatencyMs:   fp(100),
		P95LatencyMs:   fp(100),
		P99LatencyMs:   fp(100),
		AvgLatencyMs:   fp(100),
		MaxLatencyMs:   fp(100),
		ErrorRate:      fp(0.10),
		MinRPS:         fp(5),
		MinSuccessRate: fp(0.90),
	}

	result := Evaluate(th, stats, elapsed)
	if !result.Passed {
		for _, r := range result.Results {
			if !r.Passed {
				t.Errorf("threshold %s failed: target=%f actual=%f", r.Name, r.Target, r.Actual)
			}
		}
		t.Fatalf("expected all thresholds to pass")
	}
	if len(result.Results) != 9 {
		t.Fatalf("expected 9 results, got %d", len(result.Results))
	}
}

func TestEvaluate_AllThresholds_AllFail(t *testing.T) {
	latencies := make([]int, 100)
	for i := range latencies {
		latencies[i] = (i + 1) * 10 // 10..1000ms
	}
	stats := makeStats(latencies, 50, 50)
	elapsed := 100 * time.Second // 1 RPS

	th := Thresholds{
		P50LatencyMs:   fp(1),
		P90LatencyMs:   fp(1),
		P95LatencyMs:   fp(1),
		P99LatencyMs:   fp(1),
		AvgLatencyMs:   fp(1),
		MaxLatencyMs:   fp(1),
		ErrorRate:      fp(0.01),
		MinRPS:         fp(1000),
		MinSuccessRate: fp(0.99),
	}

	result := Evaluate(th, stats, elapsed)
	if result.Passed {
		t.Fatalf("expected all thresholds to fail")
	}

	for _, r := range result.Results {
		if r.Passed {
			t.Errorf("expected %s to fail, but it passed (target=%f actual=%f)", r.Name, r.Target, r.Actual)
		}
	}
}

func TestEvaluate_MixedPassFail(t *testing.T) {
	// 10 latencies: 10..100ms
	stats := makeStats([]int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 9, 1)
	elapsed := 10 * time.Second

	th := Thresholds{
		P95LatencyMs: fp(200),  // passes (actual ~90ms)
		ErrorRate:    fp(0.05), // fails (actual 0.10)
		MinRPS:       fp(1),    // passes (actual 1.0)
	}

	result := Evaluate(th, stats, elapsed)
	if result.Passed {
		t.Fatalf("expected overall fail due to error rate")
	}

	passCount := 0
	failCount := 0
	for _, r := range result.Results {
		if r.Passed {
			passCount++
		} else {
			failCount++
			if r.Name != "error_rate" {
				t.Errorf("expected error_rate to be the failing threshold, got %s", r.Name)
			}
		}
	}
	if passCount != 2 {
		t.Errorf("expected 2 passing thresholds, got %d", passCount)
	}
	if failCount != 1 {
		t.Errorf("expected 1 failing threshold, got %d", failCount)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestEvaluate_NoThresholds(t *testing.T) {
	stats := makeStats([]int{10, 20, 30}, 3, 0)
	th := Thresholds{} // all nil

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass with no thresholds configured")
	}
	if len(result.Results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(result.Results))
	}
}

func TestEvaluate_NilStats(t *testing.T) {
	th := Thresholds{
		P95LatencyMs: fp(100),
		ErrorRate:    fp(0.01),
	}

	result := Evaluate(th, nil, 10*time.Second)
	// With nil stats, all computed values are zero.
	// P95 = 0 <= 100 -> pass
	// ErrorRate = 0 <= 0.01 -> pass
	if !result.Passed {
		t.Fatalf("expected pass with nil stats (all zeros)")
	}
}

func TestEvaluate_NilStats_MinRPS(t *testing.T) {
	th := Thresholds{
		MinRPS: fp(10),
	}

	result := Evaluate(th, nil, 10*time.Second)
	// With nil stats, RPS = 0 which is < 10 -> fail
	if result.Passed {
		t.Fatalf("expected fail: min_rps requires positive RPS but stats are nil")
	}
}

func TestEvaluate_ZeroElapsed(t *testing.T) {
	stats := makeStats([]int{10}, 100, 0)
	th := Thresholds{MinRPS: fp(10)}

	result := Evaluate(th, stats, 0)
	// With zero elapsed, RPS = 0 -> fails
	if result.Passed {
		t.Fatalf("expected fail with zero elapsed")
	}
	if result.Results[0].Actual != 0 {
		t.Errorf("expected RPS 0 with zero elapsed, got %f", result.Results[0].Actual)
	}
}

func TestEvaluate_ZeroRequests(t *testing.T) {
	stats := makeStats([]int{}, 0, 0)
	th := Thresholds{
		ErrorRate:      fp(0.01),
		MinSuccessRate: fp(0.99),
	}

	result := Evaluate(th, stats, 10*time.Second)
	// Zero total: errorRate = 0/0 = 0 (pass for ErrorRate)
	// successRate = 0/0 = 0 (fail for MinSuccessRate)
	if result.Passed {
		t.Fatalf("expected fail: success rate 0 < 0.99")
	}

	for _, r := range result.Results {
		switch r.Name {
		case "error_rate":
			if !r.Passed {
				t.Errorf("error_rate should pass with 0 requests (actual 0)")
			}
		case "min_success_rate":
			if r.Passed {
				t.Errorf("min_success_rate should fail with 0 requests (actual 0)")
			}
		}
	}
}

func TestEvaluate_BoundaryValues_ExactlyAtThreshold(t *testing.T) {
	// Verify that values exactly at the threshold boundary pass.
	stats := makeStats([]int{50}, 1, 0)
	th := Thresholds{
		P50LatencyMs: fp(50),
		MaxLatencyMs: fp(50),
		AvgLatencyMs: fp(50),
	}

	result := Evaluate(th, stats, 1*time.Second)
	if !result.Passed {
		for _, r := range result.Results {
			if !r.Passed {
				t.Errorf("%s failed at boundary: target=%f actual=%f", r.Name, r.Target, r.Actual)
			}
		}
	}
}

func TestEvaluate_BoundaryValues_JustAboveThreshold(t *testing.T) {
	// Single latency of 51ms against a 50ms threshold should fail.
	stats := makeStats([]int{51}, 1, 0)
	th := Thresholds{MaxLatencyMs: fp(50)}

	result := Evaluate(th, stats, 1*time.Second)
	if result.Passed {
		t.Fatalf("expected fail: 51ms > 50ms threshold")
	}
}

func TestEvaluate_SingleLatency(t *testing.T) {
	// With one latency, all percentiles equal that value.
	stats := makeStats([]int{42}, 1, 0)
	th := Thresholds{
		P50LatencyMs: fp(42),
		P90LatencyMs: fp(42),
		P95LatencyMs: fp(42),
		P99LatencyMs: fp(42),
		AvgLatencyMs: fp(42),
		MaxLatencyMs: fp(42),
	}

	result := Evaluate(th, stats, 1*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass with single latency matching all thresholds")
	}
	for _, r := range result.Results {
		if r.Actual != 42 {
			t.Errorf("%s: expected actual 42, got %f", r.Name, r.Actual)
		}
	}
}

func TestEvaluate_EmptyLatencies_LatencyThresholds(t *testing.T) {
	// No latencies recorded but latency thresholds configured.
	// Percentiles return 0 for empty slices.
	stats := makeStats([]int{}, 0, 0)
	th := Thresholds{
		P95LatencyMs: fp(100),
		AvgLatencyMs: fp(100),
		MaxLatencyMs: fp(100),
	}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass: 0ms latencies are within 100ms thresholds")
	}
}

func TestEvaluate_LargeDataset(t *testing.T) {
	// 10000 latencies, 1-10000ms
	latencies := make([]int, 10000)
	for i := range latencies {
		latencies[i] = i + 1
	}
	stats := makeStats(latencies, 9900, 100)

	th := Thresholds{
		P99LatencyMs:   fp(10000),
		ErrorRate:      fp(0.02),
		MinSuccessRate: fp(0.98),
	}

	result := Evaluate(th, stats, 100*time.Second)
	if !result.Passed {
		for _, r := range result.Results {
			if !r.Passed {
				t.Errorf("%s failed: target=%f actual=%f", r.Name, r.Target, r.Actual)
			}
		}
	}
}

func TestEvaluate_ErrorRate_ZeroThreshold(t *testing.T) {
	// Threshold of 0 error rate: only passes if exactly 0 errors.
	stats := makeStats([]int{10}, 100, 0)
	th := Thresholds{ErrorRate: fp(0.0)}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass with zero errors and zero error rate threshold")
	}

	// Now with 1 failure out of 100
	stats2 := makeStats([]int{10}, 99, 1)
	result2 := Evaluate(th, stats2, 10*time.Second)
	if result2.Passed {
		t.Fatalf("expected fail: 1%% error rate > 0%% threshold")
	}
}

func TestEvaluate_MinSuccessRate_PerfectSuccess(t *testing.T) {
	stats := makeStats([]int{10}, 100, 0)
	th := Thresholds{MinSuccessRate: fp(1.0)}

	result := Evaluate(th, stats, 10*time.Second)
	if !result.Passed {
		t.Fatalf("expected pass with 100%% success rate")
	}
}

func TestEvaluate_ResultsOrder(t *testing.T) {
	stats := makeStats([]int{10}, 100, 0)
	th := Thresholds{
		P50LatencyMs:   fp(100),
		P90LatencyMs:   fp(100),
		P95LatencyMs:   fp(100),
		P99LatencyMs:   fp(100),
		AvgLatencyMs:   fp(100),
		MaxLatencyMs:   fp(100),
		ErrorRate:      fp(0.01),
		MinRPS:         fp(1),
		MinSuccessRate: fp(0.90),
	}

	result := Evaluate(th, stats, 10*time.Second)

	expected := []string{
		"p50_latency_ms",
		"p90_latency_ms",
		"p95_latency_ms",
		"p99_latency_ms",
		"avg_latency_ms",
		"max_latency_ms",
		"error_rate",
		"min_rps",
		"min_success_rate",
	}
	if len(result.Results) != len(expected) {
		t.Fatalf("expected %d results, got %d", len(expected), len(result.Results))
	}
	for i, name := range expected {
		if result.Results[i].Name != name {
			t.Errorf("result[%d]: expected name %q, got %q", i, name, result.Results[i].Name)
		}
	}
}

// ---------------------------------------------------------------------------
// ExitCode tests
// ---------------------------------------------------------------------------

func TestExitCode_AllPassed(t *testing.T) {
	result := EvaluationResult{
		Passed: true,
		Results: []ThresholdResult{
			{Name: "p95_latency_ms", Passed: true},
		},
	}
	if code := ExitCode(result); code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestExitCode_SomeFailed(t *testing.T) {
	result := EvaluationResult{
		Passed: false,
		Results: []ThresholdResult{
			{Name: "p95_latency_ms", Passed: true},
			{Name: "error_rate", Passed: false},
		},
	}
	if code := ExitCode(result); code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

func TestExitCode_NoThresholds(t *testing.T) {
	result := EvaluationResult{
		Passed:  true,
		Results: nil,
	}
	if code := ExitCode(result); code != 0 {
		t.Errorf("expected exit code 0 for empty thresholds, got %d", code)
	}
}

// ---------------------------------------------------------------------------
// Verify percentile computation matches converters package
// ---------------------------------------------------------------------------

func TestPercentile_Empty(t *testing.T) {
	if v := percentile(nil, 0.95); v != 0 {
		t.Errorf("expected 0 for empty slice, got %f", v)
	}
}

func TestPercentile_Single(t *testing.T) {
	lat := []time.Duration{42 * time.Millisecond}
	if v := percentile(lat, 0.99); v != 42 {
		t.Errorf("expected 42 for single element, got %f", v)
	}
}

func TestPercentile_KnownValues(t *testing.T) {
	// 10 sorted values: 10,20,30,40,50,60,70,80,90,100ms
	lat := make([]time.Duration, 10)
	for i := range lat {
		lat[i] = time.Duration((i+1)*10) * time.Millisecond
	}

	tests := []struct {
		p    float64
		want float64
	}{
		{0.00, 10},
		{0.50, 50},  // idx = int(9*0.50) = 4 -> 50
		{0.90, 90},  // idx = int(9*0.90) = 8 -> 90
		{0.95, 90},  // idx = int(9*0.95) = 8 -> 90
		{0.99, 90},  // idx = int(9*0.99) = 8 -> 90
		{1.00, 100}, // idx = int(9*1.00) = 9 -> 100
	}

	for _, tt := range tests {
		got := percentile(lat, tt.p)
		if got != tt.want {
			t.Errorf("percentile(p=%f): expected %f, got %f", tt.p, tt.want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Concurrency safety: Evaluate should not panic when mutexes are used
// ---------------------------------------------------------------------------

func TestEvaluate_ConcurrencySafety(t *testing.T) {
	stats := makeStats([]int{10, 20, 30, 40, 50}, 5, 0)
	th := Thresholds{P95LatencyMs: fp(100)}

	// Run multiple evaluations concurrently to check for races.
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_ = Evaluate(th, stats, 10*time.Second)
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
