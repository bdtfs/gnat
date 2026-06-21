// Package cli provides CLI utilities for the gnat load testing tool.
package cli

import (
	"sort"
	"time"

	"github.com/bdtfs/gnat/internal/models"
)

// Thresholds defines pass/fail criteria for a load test run.
// Each field is a pointer so that nil means "not configured" (skip this check).
type Thresholds struct {
	P50LatencyMs   *float64 `yaml:"p50_latency_ms,omitempty" json:"p50_latency_ms,omitempty"`
	P90LatencyMs   *float64 `yaml:"p90_latency_ms,omitempty" json:"p90_latency_ms,omitempty"`
	P95LatencyMs   *float64 `yaml:"p95_latency_ms,omitempty" json:"p95_latency_ms,omitempty"`
	P99LatencyMs   *float64 `yaml:"p99_latency_ms,omitempty" json:"p99_latency_ms,omitempty"`
	AvgLatencyMs   *float64 `yaml:"avg_latency_ms,omitempty" json:"avg_latency_ms,omitempty"`
	MaxLatencyMs   *float64 `yaml:"max_latency_ms,omitempty" json:"max_latency_ms,omitempty"`
	ErrorRate      *float64 `yaml:"error_rate,omitempty" json:"error_rate,omitempty"`
	MinRPS         *float64 `yaml:"min_rps,omitempty" json:"min_rps,omitempty"`
	MinSuccessRate *float64 `yaml:"min_success_rate,omitempty" json:"min_success_rate,omitempty"`
}

// ThresholdResult records the outcome of a single threshold check.
type ThresholdResult struct {
	Name   string // e.g. "p95_latency_ms"
	Target float64
	Actual float64
	Passed bool
}

// EvaluationResult holds the aggregate outcome of all threshold checks.
type EvaluationResult struct {
	Passed  bool
	Results []ThresholdResult
}

// computedStats holds values derived from models.Stats for threshold evaluation.
type computedStats struct {
	p50LatencyMs float64
	p90LatencyMs float64
	p95LatencyMs float64
	p99LatencyMs float64
	avgLatencyMs float64
	maxLatencyMs float64
	errorRate    float64
	rps          float64
	successRate  float64
}

// deriveStats computes percentiles, averages, and rates from raw models.Stats.
// elapsed is the wall-clock duration of the test run, used to compute RPS.
func deriveStats(s *models.Stats, elapsed time.Duration) computedStats {
	var cs computedStats

	if s == nil {
		return cs
	}

	// Copy and sort latencies for percentile computation.
	s.LatenciesMu.Lock()
	lat := make([]time.Duration, len(s.Latencies))
	copy(lat, s.Latencies)
	s.LatenciesMu.Unlock()

	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })

	if len(lat) > 0 {
		cs.maxLatencyMs = float64(lat[len(lat)-1].Milliseconds())

		var total time.Duration
		for _, v := range lat {
			total += v
		}
		cs.avgLatencyMs = float64(total.Milliseconds()) / float64(len(lat))
	}

	cs.p50LatencyMs = percentile(lat, 0.50)
	cs.p90LatencyMs = percentile(lat, 0.90)
	cs.p95LatencyMs = percentile(lat, 0.95)
	cs.p99LatencyMs = percentile(lat, 0.99)

	if s.TotalRequests > 0 {
		cs.errorRate = float64(s.FailedRequests) / float64(s.TotalRequests)
		cs.successRate = float64(s.SuccessRequests) / float64(s.TotalRequests)
	}

	if elapsed > 0 {
		cs.rps = float64(s.TotalRequests) / elapsed.Seconds()
	}

	return cs
}

// percentile returns the value at the given percentile p (0.0-1.0) from a
// sorted slice of durations, in milliseconds. Returns 0 for an empty slice.
func percentile(sorted []time.Duration, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return float64(sorted[idx].Milliseconds())
}

// Evaluate checks each non-nil threshold against the corresponding stat
// derived from the raw models.Stats. elapsed is the wall-clock duration of the
// test run, required for RPS computation. Overall result passes only if every
// individual threshold passes.
func Evaluate(thresholds Thresholds, stats *models.Stats, elapsed time.Duration) EvaluationResult {
	cs := deriveStats(stats, elapsed)

	var b thresholdBuilder

	b.atMost("p50_latency_ms", thresholds.P50LatencyMs, cs.p50LatencyMs)
	b.atMost("p90_latency_ms", thresholds.P90LatencyMs, cs.p90LatencyMs)
	b.atMost("p95_latency_ms", thresholds.P95LatencyMs, cs.p95LatencyMs)
	b.atMost("p99_latency_ms", thresholds.P99LatencyMs, cs.p99LatencyMs)
	b.atMost("avg_latency_ms", thresholds.AvgLatencyMs, cs.avgLatencyMs)
	b.atMost("max_latency_ms", thresholds.MaxLatencyMs, cs.maxLatencyMs)
	b.atMost("error_rate", thresholds.ErrorRate, cs.errorRate)
	b.atLeast("min_rps", thresholds.MinRPS, cs.rps)
	b.atLeast("min_success_rate", thresholds.MinSuccessRate, cs.successRate)

	return EvaluationResult{
		Passed:  b.allPassed(),
		Results: b.results,
	}
}

type thresholdBuilder struct {
	results []ThresholdResult
}

func (b *thresholdBuilder) add(name string, target, actual float64, passed bool) {
	b.results = append(b.results, ThresholdResult{
		Name:   name,
		Target: target,
		Actual: actual,
		Passed: passed,
	})
}

func (b *thresholdBuilder) atMost(name string, target *float64, actual float64) {
	if target == nil {
		return
	}
	b.add(name, *target, actual, actual <= *target)
}

func (b *thresholdBuilder) atLeast(name string, target *float64, actual float64) {
	if target == nil {
		return
	}
	b.add(name, *target, actual, actual >= *target)
}

func (b *thresholdBuilder) allPassed() bool {
	for _, r := range b.results {
		if !r.Passed {
			return false
		}
	}
	return true
}

// ExitCode maps an EvaluationResult to a process exit code.
//
//	0 = all thresholds passed
//	1 = one or more thresholds failed
func ExitCode(result EvaluationResult) int {
	if result.Passed {
		return 0
	}
	return 1
}
