package metrics

import (
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/bdtfs/gnat/internal/converters"
)

func findScenario(snaps []ScenarioStats, name string) *ScenarioStats {
	for i := range snaps {
		if snaps[i].Scenario == name {
			return &snaps[i]
		}
	}
	return nil
}

func findStep(steps []*StepStats, name string) *StepStats {
	for _, s := range steps {
		if s.Step == name {
			return s
		}
	}
	return nil
}

func TestNewSink_RingSizeDefaulting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "zero defaults", in: 0, want: defaultRingSize},
		{name: "negative defaults", in: -5, want: defaultRingSize},
		{name: "positive kept", in: 256, want: 256},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := NewSink(tt.in).(*sink)
			if s.ringSize != tt.want {
				t.Fatalf("ringSize = %d, want %d", s.ringSize, tt.want)
			}
		})
	}
}

func TestRecord_CountsAndFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		samples          []Sample
		wantCount        int64
		wantFailed       int64
		wantBytes        int64
		wantStatus       map[int]int64
		wantCheckReasons map[string]int64
	}{
		{
			name: "all good",
			samples: []Sample{
				{Scenario: "s", Step: "a", Status: 200, DurationMs: 10, BytesRead: 5, CheckPassed: true},
				{Scenario: "s", Step: "a", Status: 200, DurationMs: 20, BytesRead: 7, CheckPassed: true},
			},
			wantCount:        2,
			wantFailed:       0,
			wantBytes:        12,
			wantStatus:       map[int]int64{200: 2},
			wantCheckReasons: map[string]int64{},
		},
		{
			name: "error sample",
			samples: []Sample{
				{Scenario: "s", Step: "a", Status: 0, DurationMs: 10, CheckPassed: false, Err: errors.New("boom")},
			},
			wantCount:        1,
			wantFailed:       1,
			wantBytes:        0,
			wantStatus:       map[int]int64{},
			wantCheckReasons: map[string]int64{"error": 1},
		},
		{
			name: "check failed no error",
			samples: []Sample{
				{Scenario: "s", Step: "a", Status: 500, DurationMs: 10, CheckPassed: false},
			},
			wantCount:        1,
			wantFailed:       1,
			wantBytes:        0,
			wantStatus:       map[int]int64{500: 1},
			wantCheckReasons: map[string]int64{"check-failed": 1},
		},
		{
			name: "error wins over check reason",
			samples: []Sample{
				{Scenario: "s", Step: "a", Status: 503, DurationMs: 10, CheckPassed: false, Err: errors.New("x")},
			},
			wantCount:        1,
			wantFailed:       1,
			wantStatus:       map[int]int64{503: 1},
			wantCheckReasons: map[string]int64{"error": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sink := NewSink(100)
			for _, sm := range tt.samples {
				sink.Record(sm)
			}
			snaps := sink.Snapshot()
			sc := findScenario(snaps, "s")
			if sc == nil {
				t.Fatalf("scenario s not found")
			}
			st := findStep(sc.Steps, "a")
			if st == nil {
				t.Fatalf("step a not found")
			}
			if st.Count != tt.wantCount {
				t.Errorf("Count = %d, want %d", st.Count, tt.wantCount)
			}
			if st.Failed != tt.wantFailed {
				t.Errorf("Failed = %d, want %d", st.Failed, tt.wantFailed)
			}
			if st.BytesRead != tt.wantBytes {
				t.Errorf("BytesRead = %d, want %d", st.BytesRead, tt.wantBytes)
			}
			for code, want := range tt.wantStatus {
				if st.StatusCounts[code] != want {
					t.Errorf("StatusCounts[%d] = %d, want %d", code, st.StatusCounts[code], want)
				}
			}
			if len(st.StatusCounts) != len(tt.wantStatus) {
				t.Errorf("StatusCounts size = %d, want %d", len(st.StatusCounts), len(tt.wantStatus))
			}
			for reason, want := range tt.wantCheckReasons {
				if st.CheckFailReasons[reason] != want {
					t.Errorf("CheckFailReasons[%q] = %d, want %d", reason, st.CheckFailReasons[reason], want)
				}
			}
			if len(st.CheckFailReasons) != len(tt.wantCheckReasons) {
				t.Errorf("CheckFailReasons size = %d, want %d", len(st.CheckFailReasons), len(tt.wantCheckReasons))
			}
		})
	}
}

func TestSnapshot_PercentileIndexing(t *testing.T) {
	t.Parallel()

	sink := NewSink(1000)
	for i := 1; i <= 100; i++ {
		sink.Record(Sample{Scenario: "s", Step: "a", Status: 200, DurationMs: float64(i), CheckPassed: true})
	}
	snaps := sink.Snapshot()
	st := findStep(snaps[0].Steps, "a")

	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{name: "p50", got: st.Latency.P50, want: 50},
		{name: "p90", got: st.Latency.P90, want: 90},
		{name: "p95", got: st.Latency.P95, want: 95},
		{name: "p99", got: st.Latency.P99, want: 99},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestSnapshot_TwoStepsAndAggregate(t *testing.T) {
	t.Parallel()

	sink := NewSink(1000)
	sink.Record(Sample{Scenario: "checkout", Step: "login", Status: 200, DurationMs: 10, BytesRead: 100, CheckPassed: true})
	sink.Record(Sample{Scenario: "checkout", Step: "login", Status: 200, DurationMs: 30, BytesRead: 100, CheckPassed: true})
	sink.Record(Sample{Scenario: "checkout", Step: "pay", Status: 500, DurationMs: 50, BytesRead: 50, CheckPassed: false})

	snaps := sink.Snapshot()
	if len(snaps) != 1 {
		t.Fatalf("scenarios = %d, want 1", len(snaps))
	}
	sc := snaps[0]
	if len(sc.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(sc.Steps))
	}
	if sc.Steps[0].Step != "login" || sc.Steps[1].Step != "pay" {
		t.Errorf("steps not sorted: %s, %s", sc.Steps[0].Step, sc.Steps[1].Step)
	}
	if sc.Aggregate == nil {
		t.Fatalf("aggregate is nil")
	}
	if sc.Aggregate.Step != "*" {
		t.Errorf("aggregate step = %q, want *", sc.Aggregate.Step)
	}
	if sc.Aggregate.Count != 3 {
		t.Errorf("aggregate count = %d, want 3", sc.Aggregate.Count)
	}
	if sc.Aggregate.Failed != 1 {
		t.Errorf("aggregate failed = %d, want 1", sc.Aggregate.Failed)
	}
	if sc.Aggregate.BytesRead != 250 {
		t.Errorf("aggregate bytes = %d, want 250", sc.Aggregate.BytesRead)
	}
	if sc.Aggregate.StatusCounts[200] != 2 || sc.Aggregate.StatusCounts[500] != 1 {
		t.Errorf("aggregate status counts wrong: %v", sc.Aggregate.StatusCounts)
	}
}

func TestSnapshot_StableScenarioOrder(t *testing.T) {
	t.Parallel()

	sink := NewSink(100)
	sink.Record(Sample{Scenario: "zebra", Step: "a", Status: 200, DurationMs: 1, CheckPassed: true})
	sink.Record(Sample{Scenario: "alpha", Step: "a", Status: 200, DurationMs: 1, CheckPassed: true})
	sink.Record(Sample{Scenario: "mango", Step: "a", Status: 200, DurationMs: 1, CheckPassed: true})

	snaps := sink.Snapshot()
	want := []string{"alpha", "mango", "zebra"}
	if len(snaps) != len(want) {
		t.Fatalf("scenarios = %d, want %d", len(snaps), len(want))
	}
	for i, w := range want {
		if snaps[i].Scenario != w {
			t.Errorf("scenario[%d] = %s, want %s", i, snaps[i].Scenario, w)
		}
	}
}

func TestRecord_RingBounded(t *testing.T) {
	t.Parallel()

	const ringSize = 1000
	snk := NewSink(ringSize)
	var maxVal float64
	for i := 0; i < 10000; i++ {
		v := float64(i)
		if v > maxVal {
			maxVal = v
		}
		snk.Record(Sample{Scenario: "s", Step: "a", Status: 200, DurationMs: v, TTFBMs: v / 2, BytesRead: 1, CheckPassed: true})
	}

	s := snk.(*sink)
	s.mu.Lock()
	b := s.buckets[stepKey{scenario: "s", step: "a"}]
	latLen := len(b.latency.buf)
	ttfbLen := len(b.ttfb.buf)
	s.mu.Unlock()

	if latLen > ringSize {
		t.Errorf("latency ring len = %d, want <= %d", latLen, ringSize)
	}
	if ttfbLen > ringSize {
		t.Errorf("ttfb ring len = %d, want <= %d", ttfbLen, ringSize)
	}

	snaps := snk.Snapshot()
	st := findStep(snaps[0].Steps, "a")
	if st.Count != 10000 {
		t.Errorf("count = %d, want 10000", st.Count)
	}
	if st.Latency.P95 <= 0 {
		t.Errorf("p95 = %v, want > 0", st.Latency.P95)
	}
	if st.Latency.P95 > maxVal {
		t.Errorf("p95 = %v, want <= %v", st.Latency.P95, maxVal)
	}
}

func TestRecord_PercentilesRepresentative(t *testing.T) {
	t.Parallel()

	const ringSize = 1000
	const total = 100000
	snk := NewSink(ringSize)
	for i := 0; i < total; i++ {
		v := float64(i)
		snk.Record(Sample{Scenario: "s", Step: "a", Status: 200, DurationMs: v, CheckPassed: true})
	}

	s := snk.(*sink)
	s.mu.Lock()
	b := s.buckets[stepKey{scenario: "s", step: "a"}]
	latLen := len(b.latency.buf)
	s.mu.Unlock()
	if latLen > ringSize {
		t.Fatalf("latency ring len = %d, want <= %d", latLen, ringSize)
	}

	snaps := snk.Snapshot()
	st := findStep(snaps[0].Steps, "a")

	tol := float64(total) * 0.10
	checks := []struct {
		name string
		got  float64
		want float64
	}{
		{name: "p50", got: st.Latency.P50, want: float64(total) * 0.50},
		{name: "p90", got: st.Latency.P90, want: float64(total) * 0.90},
		{name: "p95", got: st.Latency.P95, want: float64(total) * 0.95},
		{name: "p99", got: st.Latency.P99, want: float64(total) * 0.99},
	}
	for _, c := range checks {
		if math.Abs(c.got-c.want) > tol {
			t.Errorf("%s = %v, want approx %v (tol %v)", c.name, c.got, c.want, tol)
		}
	}
}

func TestRecord_AggregateRepresentative(t *testing.T) {
	t.Parallel()

	const ringSize = 1000
	const perStep = 50000
	snk := NewSink(ringSize)
	for i := 0; i < perStep; i++ {
		v := float64(i)
		snk.Record(Sample{Scenario: "s", Step: "a", Status: 200, DurationMs: v, CheckPassed: true})
		snk.Record(Sample{Scenario: "s", Step: "b", Status: 200, DurationMs: v, CheckPassed: true})
	}

	snaps := snk.Snapshot()
	agg := snaps[0].Aggregate
	if len(agg.latencySamples) > 2*ringSize {
		t.Fatalf("aggregate samples = %d, want <= %d", len(agg.latencySamples), 2*ringSize)
	}

	want := float64(perStep) * 0.50
	tol := float64(perStep) * 0.10
	if math.Abs(agg.Latency.P50-want) > tol {
		t.Errorf("aggregate p50 = %v, want approx %v (tol %v)", agg.Latency.P50, want, tol)
	}
}

func TestRecord_Concurrent(t *testing.T) {
	t.Parallel()

	sink := NewSink(5000)
	var wg sync.WaitGroup
	const goroutines = 20
	const perG = 1000
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				sink.Record(Sample{Scenario: "s", Step: "a", Status: 200, DurationMs: float64(i), CheckPassed: true})
			}
		}()
	}
	wg.Wait()

	snaps := sink.Snapshot()
	st := findStep(snaps[0].Steps, "a")
	if st.Count != goroutines*perG {
		t.Errorf("count = %d, want %d", st.Count, goroutines*perG)
	}
}

func TestToModelsStats_RoundTrip(t *testing.T) {
	t.Parallel()

	sink := NewSink(1000)
	for i := 1; i <= 200; i++ {
		passed := i%10 != 0
		sink.Record(Sample{Scenario: "s", Step: "a", Status: 200, DurationMs: float64(i), CheckPassed: passed})
	}
	snaps := sink.Snapshot()
	st := findStep(snaps[0].Steps, "a")

	ms := st.ToModelsStats()
	if ms == nil {
		t.Fatalf("ToModelsStats returned nil")
	}
	if ms.TotalRequests != uint64(st.Count) {
		t.Errorf("TotalRequests = %d, want %d", ms.TotalRequests, st.Count)
	}
	if ms.FailedRequests != uint64(st.Failed) {
		t.Errorf("FailedRequests = %d, want %d", ms.FailedRequests, st.Failed)
	}
	if ms.SuccessRequests != uint64(st.Count-st.Failed) {
		t.Errorf("SuccessRequests = %d, want %d", ms.SuccessRequests, st.Count-st.Failed)
	}
	if len(ms.Latencies) != len(st.latencySamples) {
		t.Errorf("Latencies len = %d, want %d", len(ms.Latencies), len(st.latencySamples))
	}

	start := time.Now()
	end := start.Add(time.Second)
	dtoStats := converters.StatsToDTO(ms, start, end)
	if dtoStats == nil {
		t.Fatalf("StatsToDTO returned nil")
	}
	if dtoStats.Total != uint64(st.Count) {
		t.Errorf("dto Total = %d, want %d", dtoStats.Total, st.Count)
	}
	if dtoStats.P95Latency <= 0 {
		t.Errorf("dto P95 = %v, want > 0", dtoStats.P95Latency)
	}
}

func TestToModelsStats_NilReceiver(t *testing.T) {
	t.Parallel()

	var st *StepStats
	if st.ToModelsStats() != nil {
		t.Errorf("nil receiver should return nil")
	}
}
