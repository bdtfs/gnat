package main

import (
	"testing"
	"time"

	"github.com/bdtfs/gnat/internal/cli"
	"github.com/bdtfs/gnat/internal/metrics"
	"github.com/bdtfs/gnat/internal/scenario"
)

func ptrFloat(v float64) *float64 { return &v }

func TestBuildReportSurfacesTTFB(t *testing.T) {
	snap := []metrics.ScenarioStats{
		{
			Scenario: "s1",
			Steps: []*metrics.StepStats{
				{
					Scenario: "s1",
					Step:     "step1",
					Count:    10,
					Failed:   0,
					TTFB:     metrics.Percentiles{P50: 12, P95: 34, P99: 56},
				},
			},
		},
	}
	cfg := &scenario.Config{Name: "cfg"}
	start := time.Now()
	end := start.Add(time.Second)

	report := buildReport(cfg, snap, start, end)

	if len(report.Scenarios) != 1 || len(report.Scenarios[0].Steps) != 1 {
		t.Fatalf("unexpected report shape: %+v", report)
	}
	st := report.Scenarios[0].Steps[0].Stats
	if st == nil {
		t.Fatalf("step stats nil")
	}
	if st.TTFBP50Latency != 12 {
		t.Errorf("TTFB p50 = %v, want 12", st.TTFBP50Latency)
	}
	if st.TTFBP95Latency != 34 {
		t.Errorf("TTFB p95 = %v, want 34", st.TTFBP95Latency)
	}
	if st.TTFBP99Latency != 56 {
		t.Errorf("TTFB p99 = %v, want 56", st.TTFBP99Latency)
	}
}

func TestBuildReportRollsStepFailuresIntoScenario(t *testing.T) {
	snap := []metrics.ScenarioStats{
		{
			Scenario: "s1",
			Steps: []*metrics.StepStats{
				{Scenario: "s1", Step: "step1", Count: 10, Failed: 3},
			},
		},
	}
	cfg := &scenario.Config{Name: "cfg"}
	start := time.Now()
	end := start.Add(time.Second)

	report := buildReport(cfg, snap, start, end)

	if report.Scenarios[0].Passed {
		t.Errorf("scenario should not pass when a step failed")
	}
	if report.Passed {
		t.Errorf("report should not pass when a step failed")
	}
}

func TestBuildReportScenarioLevelThresholds(t *testing.T) {
	snap := []metrics.ScenarioStats{
		{
			Scenario: "s1",
			Steps: []*metrics.StepStats{
				{Scenario: "s1", Step: "aaa", Count: 5, Failed: 0},
				{Scenario: "s1", Step: "zzz", Count: 5, Failed: 0},
			},
			Aggregate: &metrics.StepStats{Scenario: "s1", Step: "s1", Count: 10, Failed: 0},
		},
	}
	cfg := &scenario.Config{
		Name:       "cfg",
		Thresholds: &cli.Thresholds{MinSuccessRate: ptrFloat(0.5)},
	}
	start := time.Now()
	end := start.Add(time.Second)

	report := buildReport(cfg, snap, start, end)

	sr := report.Scenarios[0]
	if len(sr.Thresholds) == 0 {
		t.Fatalf("expected scenario-level thresholds to be populated")
	}
	for _, st := range sr.Steps {
		if len(st.Thresholds) != 0 {
			t.Errorf("step %q should not carry scenario-aggregate thresholds", st.Name)
		}
	}
}
