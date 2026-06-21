package checks

import (
	"testing"
)

func TestStatusAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		spec   Spec
		status int
		want   bool
	}{
		{name: "default accepts 200", spec: DefaultSpec(), status: 200, want: true},
		{name: "default accepts 399", spec: DefaultSpec(), status: 399, want: true},
		{name: "default rejects 400", spec: DefaultSpec(), status: 400, want: false},
		{name: "default rejects 500", spec: DefaultSpec(), status: 500, want: false},
		{name: "default rejects 199", spec: DefaultSpec(), status: 199, want: false},
		{name: "list accepts member 206", spec: Spec{ExpectStatus: []int{200, 206}}, status: 206, want: true},
		{name: "list accepts member 200", spec: Spec{ExpectStatus: []int{200, 206}}, status: 200, want: true},
		{name: "list rejects non-member", spec: Spec{ExpectStatus: []int{200, 206}}, status: 201, want: false},
		{name: "list rejects 200-range default fallback off", spec: Spec{ExpectStatus: []int{404}}, status: 200, want: false},
		{name: "range accepts low bound", spec: Spec{ExpectStatusRange: [2]int{500, 599}}, status: 500, want: true},
		{name: "range accepts high bound", spec: Spec{ExpectStatusRange: [2]int{500, 599}}, status: 599, want: true},
		{name: "range accepts inside", spec: Spec{ExpectStatusRange: [2]int{500, 599}}, status: 503, want: true},
		{name: "range rejects below", spec: Spec{ExpectStatusRange: [2]int{500, 599}}, status: 499, want: false},
		{name: "range rejects above", spec: Spec{ExpectStatusRange: [2]int{500, 599}}, status: 600, want: false},
		{name: "both list match", spec: Spec{ExpectStatus: []int{200}, ExpectStatusRange: [2]int{500, 599}}, status: 200, want: true},
		{name: "both range match", spec: Spec{ExpectStatus: []int{200}, ExpectStatusRange: [2]int{500, 599}}, status: 503, want: true},
		{name: "both neither match", spec: Spec{ExpectStatus: []int{200}, ExpectStatusRange: [2]int{500, 599}}, status: 404, want: false},
		{name: "partial range lo only ignored", spec: Spec{ExpectStatusRange: [2]int{500, 0}}, status: 200, want: true},
		{name: "partial range hi only ignored", spec: Spec{ExpectStatusRange: [2]int{0, 599}}, status: 200, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := StatusAllowed(tt.spec, tt.status); got != tt.want {
				t.Errorf("StatusAllowed(%+v, %d) = %v, want %v", tt.spec, tt.status, got, tt.want)
			}
		})
	}
}

func TestDefaultSpec(t *testing.T) {
	t.Parallel()

	spec := DefaultSpec()

	if spec.Required {
		t.Errorf("expected DefaultSpec Required false, got true")
	}
	if len(spec.ExpectStatus) != 0 {
		t.Errorf("expected empty ExpectStatus, got %v", spec.ExpectStatus)
	}
	if spec.ExpectStatusRange != [2]int{0, 0} {
		t.Errorf("expected zero ExpectStatusRange, got %v", spec.ExpectStatusRange)
	}
	if !StatusAllowed(spec, 200) {
		t.Errorf("expected DefaultSpec to accept 200")
	}
	if !StatusAllowed(spec, 399) {
		t.Errorf("expected DefaultSpec to accept 399")
	}
	if StatusAllowed(spec, 400) {
		t.Errorf("expected DefaultSpec to reject 400")
	}
	if StatusAllowed(spec, 500) {
		t.Errorf("expected DefaultSpec to reject 500")
	}
}

func TestEvaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		spec        Spec
		status      int
		durationMs  float64
		ttfbMs      float64
		body        []byte
		wantPassed  bool
		wantAbort   bool
		wantResults int
		checkResult map[string]bool
	}{
		{
			name:        "default success 200",
			spec:        DefaultSpec(),
			status:      200,
			wantPassed:  true,
			wantAbort:   false,
			wantResults: 1,
			checkResult: map[string]bool{"status": true},
		},
		{
			name:        "expect list passes 206",
			spec:        Spec{ExpectStatus: []int{200, 206}},
			status:      206,
			wantPassed:  true,
			wantAbort:   false,
			wantResults: 1,
			checkResult: map[string]bool{"status": true},
		},
		{
			name:        "required 500 fails and aborts",
			spec:        Spec{Required: true},
			status:      500,
			wantPassed:  false,
			wantAbort:   true,
			wantResults: 1,
			checkResult: map[string]bool{"status": false},
		},
		{
			name:        "non-required 500 fails no abort",
			spec:        Spec{Required: false},
			status:      500,
			wantPassed:  false,
			wantAbort:   false,
			wantResults: 1,
			checkResult: map[string]bool{"status": false},
		},
		{
			name:        "max duration violated",
			spec:        Spec{MaxDurationMs: 100},
			status:      200,
			durationMs:  150,
			wantPassed:  false,
			wantAbort:   false,
			wantResults: 2,
			checkResult: map[string]bool{"status": true, "max_duration": false},
		},
		{
			name:        "max duration within limit",
			spec:        Spec{MaxDurationMs: 100},
			status:      200,
			durationMs:  100,
			wantPassed:  true,
			wantAbort:   false,
			wantResults: 2,
			checkResult: map[string]bool{"status": true, "max_duration": true},
		},
		{
			name:        "max ttfb violated",
			spec:        Spec{MaxTTFBMs: 50},
			status:      200,
			ttfbMs:      75,
			wantPassed:  false,
			wantAbort:   false,
			wantResults: 2,
			checkResult: map[string]bool{"status": true, "max_ttfb": false},
		},
		{
			name:        "max ttfb within limit",
			spec:        Spec{MaxTTFBMs: 50},
			status:      200,
			ttfbMs:      40,
			wantPassed:  true,
			wantAbort:   false,
			wantResults: 2,
			checkResult: map[string]bool{"status": true, "max_ttfb": true},
		},
		{
			name:        "body contains violated",
			spec:        Spec{BodyContains: "ok"},
			status:      200,
			body:        []byte("error response"),
			wantPassed:  false,
			wantAbort:   false,
			wantResults: 2,
			checkResult: map[string]bool{"status": true, "body_contains": false},
		},
		{
			name:        "body contains satisfied",
			spec:        Spec{BodyContains: "ok"},
			status:      200,
			body:        []byte("status ok here"),
			wantPassed:  true,
			wantAbort:   false,
			wantResults: 2,
			checkResult: map[string]bool{"status": true, "body_contains": true},
		},
		{
			name:        "all checks required and failing aborts",
			spec:        Spec{MaxDurationMs: 100, MaxTTFBMs: 50, BodyContains: "ok", Required: true},
			status:      500,
			durationMs:  200,
			ttfbMs:      100,
			body:        []byte("nope"),
			wantPassed:  false,
			wantAbort:   true,
			wantResults: 4,
			checkResult: map[string]bool{"status": false, "max_duration": false, "max_ttfb": false, "body_contains": false},
		},
		{
			name:        "all checks passing",
			spec:        Spec{MaxDurationMs: 100, MaxTTFBMs: 50, BodyContains: "ok"},
			status:      200,
			durationMs:  50,
			ttfbMs:      20,
			body:        []byte("ok"),
			wantPassed:  true,
			wantAbort:   false,
			wantResults: 4,
			checkResult: map[string]bool{"status": true, "max_duration": true, "max_ttfb": true, "body_contains": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := Evaluate(tt.spec, tt.status, tt.durationMs, tt.ttfbMs, tt.body)

			if out.Passed != tt.wantPassed {
				t.Errorf("Passed = %v, want %v", out.Passed, tt.wantPassed)
			}
			if out.AbortVU != tt.wantAbort {
				t.Errorf("AbortVU = %v, want %v", out.AbortVU, tt.wantAbort)
			}
			if len(out.Results) != tt.wantResults {
				t.Fatalf("len(Results) = %d, want %d (%+v)", len(out.Results), tt.wantResults, out.Results)
			}

			got := make(map[string]bool, len(out.Results))
			for _, r := range out.Results {
				got[r.Name] = r.Passed
				if r.Required != tt.spec.Required {
					t.Errorf("result %q Required = %v, want %v", r.Name, r.Required, tt.spec.Required)
				}
				if !r.Passed && r.Reason == "" {
					t.Errorf("result %q failed but Reason is empty", r.Name)
				}
				if r.Passed && r.Reason != "" {
					t.Errorf("result %q passed but Reason is %q", r.Name, r.Reason)
				}
			}
			for name, want := range tt.checkResult {
				if got[name] != want {
					t.Errorf("result %q Passed = %v, want %v", name, got[name], want)
				}
			}
		})
	}
}

func TestEvaluateResultOrder(t *testing.T) {
	t.Parallel()

	spec := Spec{MaxDurationMs: 100, MaxTTFBMs: 50, BodyContains: "ok"}
	out := Evaluate(spec, 200, 10, 5, []byte("ok"))

	wantOrder := []string{"status", "max_duration", "max_ttfb", "body_contains"}
	if len(out.Results) != len(wantOrder) {
		t.Fatalf("len(Results) = %d, want %d", len(out.Results), len(wantOrder))
	}
	for i, name := range wantOrder {
		if out.Results[i].Name != name {
			t.Errorf("Results[%d].Name = %q, want %q", i, out.Results[i].Name, name)
		}
	}
}
