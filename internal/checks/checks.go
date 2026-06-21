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

func statusInList(list []int, status int) bool {
	for _, s := range list {
		if s == status {
			return true
		}
	}
	return false
}

func statusInRange(r [2]int, status int) bool {
	return status >= r[0] && status <= r[1]
}

func StatusAllowed(spec Spec, status int) bool {
	hasList := len(spec.ExpectStatus) > 0
	hasRange := spec.ExpectStatusRange[0] > 0 && spec.ExpectStatusRange[1] > 0

	if !hasList && !hasRange {
		return status >= 200 && status <= 399
	}

	if hasList && statusInList(spec.ExpectStatus, status) {
		return true
	}

	if hasRange && statusInRange(spec.ExpectStatusRange, status) {
		return true
	}

	return false
}

func newResult(name string, spec Spec, ok bool, reason string) Result {
	if ok {
		reason = ""
	}
	return Result{
		Name:     name,
		Reason:   reason,
		Passed:   ok,
		Required: spec.Required,
	}
}

func evalStatus(spec Spec, status int) Result {
	ok := StatusAllowed(spec, status)
	return newResult("status", spec, ok, fmt.Sprintf("unexpected status %d", status))
}

func evalMaxDuration(spec Spec, durationMs float64) (Result, bool) {
	if spec.MaxDurationMs <= 0 {
		return Result{}, false
	}
	ok := durationMs <= spec.MaxDurationMs
	reason := fmt.Sprintf("duration %.2fms exceeds max %.2fms", durationMs, spec.MaxDurationMs)
	return newResult("max_duration", spec, ok, reason), true
}

func evalMaxTTFB(spec Spec, ttfbMs float64) (Result, bool) {
	if spec.MaxTTFBMs <= 0 {
		return Result{}, false
	}
	ok := ttfbMs <= spec.MaxTTFBMs
	reason := fmt.Sprintf("ttfb %.2fms exceeds max %.2fms", ttfbMs, spec.MaxTTFBMs)
	return newResult("max_ttfb", spec, ok, reason), true
}

func evalBodyContains(spec Spec, body []byte) (Result, bool) {
	if spec.BodyContains == "" {
		return Result{}, false
	}
	ok := bytes.Contains(body, []byte(spec.BodyContains))
	reason := fmt.Sprintf("body does not contain %q", spec.BodyContains)
	return newResult("body_contains", spec, ok, reason), true
}

func Evaluate(spec Spec, status int, durationMs, ttfbMs float64, body []byte) Outcome {
	results := make([]Result, 0, 4)
	results = append(results, evalStatus(spec, status))

	if r, ok := evalMaxDuration(spec, durationMs); ok {
		results = append(results, r)
	}
	if r, ok := evalMaxTTFB(spec, ttfbMs); ok {
		results = append(results, r)
	}
	if r, ok := evalBodyContains(spec, body); ok {
		results = append(results, r)
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
