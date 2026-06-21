package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/bdtfs/gnat/internal/cli"
	"github.com/bdtfs/gnat/internal/converters"
	"github.com/bdtfs/gnat/internal/executor"
	"github.com/bdtfs/gnat/internal/metrics"
	"github.com/bdtfs/gnat/internal/scenario"
	"github.com/bdtfs/gnat/internal/server/dto"
	"github.com/bdtfs/gnat/internal/vu"
	httpclient "github.com/bdtfs/gnat/pkg/clients/http"
)

type runFlags struct {
	config string
	out    string
	quiet  bool
}

type StepReport struct {
	Name       string                `json:"name"`
	Stats      *dto.Stats            `json:"stats"`
	Checks     CheckSummary          `json:"checks"`
	Thresholds []cli.ThresholdResult `json:"thresholds,omitempty"`
	Passed     bool                  `json:"passed"`
}

type CheckSummary struct {
	Total    int64            `json:"total"`
	Failed   int64            `json:"failed"`
	ByReason map[string]int64 `json:"by_reason,omitempty"`
}

type RunScenario struct {
	Name   string       `json:"name"`
	Steps  []StepReport `json:"steps"`
	Passed bool         `json:"passed"`
}

type RunReport struct {
	Name        string        `json:"name"`
	GeneratedAt string        `json:"generated_at"`
	Passed      bool          `json:"passed"`
	Scenarios   []RunScenario `json:"scenarios"`
}

func parseRunFlags(args []string) (runFlags, error) {
	var f runFlags
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
			return f, fmt.Errorf("unknown run flag %q", args[i])
		}
	}
	if f.config == "" {
		return f, fmt.Errorf("--config is required")
	}
	return f, nil
}

func runRun(args []string) int {
	flags, err := parseRunFlags(args)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n\n", err)
		printUsage(os.Stderr)
		return failExitCode
	}

	cfg, err := scenario.Load(flags.config)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return failExitCode
	}

	factory := vu.NewFactory(httpclient.DefaultConfig(), cfg.Variables)

	var plans []executor.Plan
	for i := range cfg.Scenarios {
		exp, err := cfg.Scenarios[i].Expand()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: scenario %q: %v\n", cfg.Scenarios[i].Name, err)
			return failExitCode
		}
		plans = append(plans, executor.Plan{
			Name:     exp.Name,
			Flow:     exp.Flow,
			Identity: exp.Identity,
			Cfg:      exp.Cfg,
			Weight:   exp.Weight,
			Factory:  factory,
		})
	}

	sink := metrics.NewSink(10000)
	ctx := context.Background()

	start := time.Now()
	_ = executor.RunPlans(ctx, plans, sink)
	end := time.Now()

	report := buildReport(cfg, sink.Snapshot(), start, end)

	if !flags.quiet {
		printRunReport(report)
	}
	if flags.out != "" {
		if err := writeRunJSON(flags.out, report); err != nil {
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

func buildReport(cfg *scenario.Config, snap []metrics.ScenarioStats, start, end time.Time) RunReport {
	elapsed := end.Sub(start)
	report := RunReport{
		Name:        cfg.Name,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Passed:      true,
	}
	for _, sc := range snap {
		sr := RunScenario{Name: sc.Scenario, Passed: true}
		for _, step := range sc.Steps {
			stepRep := StepReport{
				Name:   step.Step,
				Stats:  converters.StatsToDTO(step.ToModelsStats(), start, end),
				Checks: CheckSummary{Total: step.Count, Failed: step.Failed, ByReason: step.CheckFailReasons},
				Passed: step.Failed == 0,
			}
			sr.Steps = append(sr.Steps, stepRep)
		}
		if cfg.Thresholds != nil && sc.Aggregate != nil {
			eval := cli.Evaluate(*cfg.Thresholds, sc.Aggregate.ToModelsStats(), elapsed)
			if len(sr.Steps) > 0 {
				sr.Steps[len(sr.Steps)-1].Thresholds = eval.Results
			}
			if !eval.Passed {
				sr.Passed = false
			}
		}
		if !sr.Passed {
			report.Passed = false
		}
		report.Scenarios = append(report.Scenarios, sr)
	}
	return report
}

func printRunReport(r RunReport) {
	for _, sc := range r.Scenarios {
		fmt.Printf("\n══ scenario: %s ══\n", sc.Name)
		for _, st := range sc.Steps {
			s := st.Stats
			result := "PASS"
			if !st.Passed {
				result = "FAIL"
			}
			fmt.Printf("  [%s] %s\n", result, st.Name)
			if s != nil {
				fmt.Printf("      reqs=%d ok=%d fail=%d  rps=%.1f  ms: p50=%.1f p95=%.1f p99=%.1f max=%.1f  bytes=%d\n",
					s.Total, s.Success, s.Failed, s.RPS, s.P50Latency, s.P95Latency, s.P99Latency, s.MaxLatency, s.BytesRead)
				if len(s.StatusCodes) > 0 {
					codes := make([]int, 0, len(s.StatusCodes))
					for c := range s.StatusCodes {
						codes = append(codes, c)
					}
					sort.Ints(codes)
					fmt.Printf("      status:")
					for _, c := range codes {
						fmt.Printf(" %d=%d", c, s.StatusCodes[c])
					}
					fmt.Println()
				}
			}
			if st.Checks.Failed > 0 && len(st.Checks.ByReason) > 0 {
				fmt.Printf("      check failures:")
				for reason, n := range st.Checks.ByReason {
					fmt.Printf(" %s=%d", reason, n)
				}
				fmt.Println()
			}
			for _, t := range st.Thresholds {
				mark := "PASS"
				if !t.Passed {
					mark = "FAIL"
				}
				fmt.Printf("      [%s] threshold %s target=%.2f actual=%.2f\n", mark, t.Name, t.Target, t.Actual)
			}
		}
	}
	overall := "PASS"
	if !r.Passed {
		overall = "FAIL"
	}
	fmt.Printf("\n=> %s\n", overall)
}

func writeRunJSON(path string, r RunReport) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
