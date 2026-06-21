package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/bdtfs/gnat/internal/cli"
	"github.com/bdtfs/gnat/internal/converters"
	"github.com/bdtfs/gnat/internal/models"
	"github.com/bdtfs/gnat/internal/runner"
	"github.com/bdtfs/gnat/internal/server/dto"
	memorystorage "github.com/bdtfs/gnat/internal/storage/memory"

	"log/slog"
)

type attackFlags struct {
	config string
	out    string
	quiet  bool
}

// ScenarioReport is the machine-readable result for a single scenario.
type ScenarioReport struct {
	Name       string                `json:"name"`
	Method     string                `json:"method"`
	URL        string                `json:"url"`
	RPS        int                   `json:"rps"`
	Duration   string                `json:"duration"`
	Status     string                `json:"status"`
	Error      string                `json:"error,omitempty"`
	Stats      *dto.Stats            `json:"stats,omitempty"`
	Thresholds []cli.ThresholdResult `json:"thresholds,omitempty"`
	Passed     bool                  `json:"passed"`
}

// AttackReport is the top-level machine-readable result of an attack run.
type AttackReport struct {
	Name        string           `json:"name"`
	GeneratedAt string           `json:"generated_at"`
	Passed      bool             `json:"passed"`
	Scenarios   []ScenarioReport `json:"scenarios"`
}

func parseAttackFlags(args []string) (attackFlags, error) {
	var f attackFlags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config", "-c":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--config requires a value")
			}
			i++
			f.config = args[i]
		case "--out", "-o":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--out requires a value")
			}
			i++
			f.out = args[i]
		case "--quiet", "-q":
			f.quiet = true
		default:
			return f, fmt.Errorf("unknown attack flag %q", args[i])
		}
	}
	if f.config == "" {
		return f, fmt.Errorf("--config is required")
	}
	return f, nil
}

func runAttack(args []string) int {
	flags, err := parseAttackFlags(args)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n\n", err)
		printUsage(os.Stderr)
		return failExitCode
	}

	cfg, err := cli.LoadConfig(flags.config)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return failExitCode
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exec := newAttackExecutor()

	report := AttackReport{
		Name:        cfg.Name,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Passed:      true,
	}

	for _, sc := range cfg.Scenarios {
		sr := exec.runScenario(ctx, sc, cfg.Thresholds)
		if !sr.Passed {
			report.Passed = false
		}
		report.Scenarios = append(report.Scenarios, sr)
		if !flags.quiet {
			printScenarioReport(sr)
		}
	}

	if flags.out != "" {
		if err := writeReport(flags.out, report); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error writing results: %v\n", err)
			return failExitCode
		}
		if !flags.quiet {
			fmt.Printf("\nResults written to %s\n", flags.out)
		}
	}

	if !report.Passed {
		return failExitCode
	}
	return successExitCode
}

type attackExecutor struct {
	repo      *memorystorage.Repository
	collector *runner.Collector
	runner    *runner.Runner

	mu    sync.Mutex
	waits map[string]chan models.RunStatus
}

func newAttackExecutor() *attackExecutor {
	repo := memorystorage.New()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	collector := runner.NewCollector()
	r := runner.New(repo, logger, collector)

	e := &attackExecutor{
		repo:      repo,
		collector: collector,
		runner:    r,
		waits:     make(map[string]chan models.RunStatus),
	}

	r.OnRunComplete(func(runID string, status models.RunStatus) {
		e.mu.Lock()
		ch, ok := e.waits[runID]
		e.mu.Unlock()
		if ok {
			ch <- status
		}
	})

	return e
}

func (e *attackExecutor) runScenario(ctx context.Context, sc cli.Scenario, thresholds *cli.Thresholds) ScenarioReport {
	sr := ScenarioReport{
		Name:     sc.Name,
		Method:   sc.Method,
		URL:      sc.URL,
		RPS:      sc.RPS,
		Duration: sc.Duration,
	}

	dur, err := sc.ParseDuration()
	if err != nil {
		sr.Status = "failed"
		sr.Error = err.Error()
		return sr
	}

	var body []byte
	if sc.Body != "" {
		body = []byte(sc.Body)
	}

	setup := models.NewSetup(sc.Name, "", sc.Method, sc.URL, body, sc.Headers, sc.RPS, dur)
	if err := e.repo.CreateSetup(setup); err != nil {
		sr.Status = "failed"
		sr.Error = err.Error()
		return sr
	}

	done := make(chan models.RunStatus, 1)
	run, err := e.runner.StartRun(ctx, setup.ID)
	if err != nil {
		sr.Status = "failed"
		sr.Error = err.Error()
		return sr
	}

	e.mu.Lock()
	e.waits[run.ID] = done
	e.mu.Unlock()

	<-done

	e.mu.Lock()
	delete(e.waits, run.ID)
	e.mu.Unlock()

	sr.Status = string(run.Status)
	if run.Error != "" {
		sr.Error = run.Error
	}

	if run.Stats != nil {
		sr.Stats = converters.StatsToDTO(run.Stats, run.StartedAt, run.EndedAt)
	}

	sr.Passed = run.Status == models.RunStatusCompleted
	if thresholds != nil && run.Stats != nil {
		elapsed := run.EndedAt.Sub(run.StartedAt)
		eval := cli.Evaluate(*thresholds, run.Stats, elapsed)
		sr.Thresholds = eval.Results
		if !eval.Passed {
			sr.Passed = false
		}
	}

	return sr
}

func printScenarioReport(sr ScenarioReport) {
	fmt.Printf("\n── %s ──\n", sr.Name)
	fmt.Printf("  %s %s  (rps=%d, duration=%s)\n", sr.Method, sr.URL, sr.RPS, sr.Duration)
	fmt.Printf("  status: %s\n", sr.Status)
	if sr.Error != "" {
		fmt.Printf("  error: %s\n", sr.Error)
	}
	if sr.Stats != nil {
		s := sr.Stats
		fmt.Printf("  requests: %d total, %d ok, %d failed (%.2f%% success)\n",
			s.Total, s.Success, s.Failed, s.SuccessRate*100)
		fmt.Printf("  throughput: %.1f req/s, %d bytes read\n", s.RPS, s.BytesRead)
		fmt.Printf("  latency ms: avg=%.1f p50=%.1f p90=%.1f p95=%.1f p99=%.1f max=%.1f\n",
			s.AvgLatency, s.P50Latency, s.P90Latency, s.P95Latency, s.P99Latency, s.MaxLatency)
		if len(s.StatusCodes) > 0 {
			codes := make([]int, 0, len(s.StatusCodes))
			for c := range s.StatusCodes {
				codes = append(codes, c)
			}
			sort.Ints(codes)
			fmt.Printf("  status codes:")
			for _, c := range codes {
				fmt.Printf(" %d=%d", c, s.StatusCodes[c])
			}
			fmt.Println()
		}
	}
	if len(sr.Thresholds) > 0 {
		fmt.Printf("  thresholds:\n")
		for _, t := range sr.Thresholds {
			mark := "PASS"
			if !t.Passed {
				mark = "FAIL"
			}
			fmt.Printf("    [%s] %s target=%.2f actual=%.2f\n", mark, t.Name, t.Target, t.Actual)
		}
	}
	result := "PASS"
	if !sr.Passed {
		result = "FAIL"
	}
	fmt.Printf("  => %s\n", result)
}

func writeReport(path string, report AttackReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
