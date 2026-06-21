package checks

import (
	"bytes"
	"fmt"
)

type Spec struct {
	ExpectStatus      []int
	ExpectStatusRange [2]int
	MaxDurationMs     float64
	MaxTTFBMs         float64
	BodyContains      string
	Required          bool
}

type Result struct {
	Name     string
	Reason   string
	Passed   bool
	Required bool
}

type Outcome struct {
	Passed  bool
	AbortVU bool
	Results []Result
}

func DefaultSpec() Spec {
	return Spec{}
}

func StatusAllowed(spec Spec, status int) bool {
	hasList := len(spec.ExpectStatus) > 0
	hasRange := spec.ExpectStatusRange[0] > 0 && spec.ExpectStatusRange[1] > 0

	if !hasList && !hasRange {
		return status >= 200 && status <= 399
	}

	if hasList {
		for _, s := range spec.ExpectStatus {
			if s == status {
				return true
			}
		}
	}

	if hasRange {
		if status >= spec.ExpectStatusRange[0] && status <= spec.ExpectStatusRange[1] {
			return true
		}
	}

	return false
}

func Evaluate(spec Spec, status int, durationMs, ttfbMs float64, body []byte) Outcome {
	results := make([]Result, 0, 4)

	statusOK := StatusAllowed(spec, status)
	statusReason := ""
	if !statusOK {
		statusReason = fmt.Sprintf("unexpected status %d", status)
	}
	results = append(results, Result{
		Name:     "status",
		Reason:   statusReason,
		Passed:   statusOK,
		Required: spec.Required,
	})

	if spec.MaxDurationMs > 0 {
		ok := durationMs <= spec.MaxDurationMs
		reason := ""
		if !ok {
			reason = fmt.Sprintf("duration %.2fms exceeds max %.2fms", durationMs, spec.MaxDurationMs)
		}
		results = append(results, Result{
			Name:     "max_duration",
			Reason:   reason,
			Passed:   ok,
			Required: spec.Required,
		})
	}

	if spec.MaxTTFBMs > 0 {
		ok := ttfbMs <= spec.MaxTTFBMs
		reason := ""
		if !ok {
			reason = fmt.Sprintf("ttfb %.2fms exceeds max %.2fms", ttfbMs, spec.MaxTTFBMs)
		}
		results = append(results, Result{
			Name:     "max_ttfb",
			Reason:   reason,
			Passed:   ok,
			Required: spec.Required,
		})
	}

	if spec.BodyContains != "" {
		ok := bytes.Contains(body, []byte(spec.BodyContains))
		reason := ""
		if !ok {
			reason = fmt.Sprintf("body does not contain %q", spec.BodyContains)
		}
		results = append(results, Result{
			Name:     "body_contains",
			Reason:   reason,
			Passed:   ok,
			Required: spec.Required,
		})
	}

	passed := true
	for _, r := range results {
		if !r.Passed {
			passed = false
			break
		}
	}

	return Outcome{
		Passed:  passed,
		AbortVU: !passed && spec.Required,
		Results: results,
	}
}
