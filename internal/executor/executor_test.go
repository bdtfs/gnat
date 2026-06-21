package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bdtfs/gnat/internal/checks"
	"github.com/bdtfs/gnat/internal/metrics"
	"github.com/bdtfs/gnat/internal/vu"
	httpclient "github.com/bdtfs/gnat/pkg/clients/http"
)

func TestVUCountAt(t *testing.T) {
	t.Parallel()
	cfg := VUConfig{
		Type: "ramping-vus",
		Stages: []Stage{
			{Target: 20, Duration: 30 * time.Second},
			{Target: 20, Duration: 2 * time.Minute},
			{Target: 0, Duration: 30 * time.Second},
		},
	}
	cases := []struct {
		elapsed time.Duration
		want    int
	}{
		{0, 0},
		{15 * time.Second, 10},
		{30 * time.Second, 20},
		{90 * time.Second, 20},
		{150 * time.Second, 20},
		{165 * time.Second, 10},
		{180 * time.Second, 0},
		{200 * time.Second, 0},
	}
	for _, c := range cases {
		if got := vuCountAt(cfg, c.elapsed); got != c.want {
			t.Errorf("vuCountAt(%s) = %d, want %d", c.elapsed, got, c.want)
		}
	}
}

func TestRunConstantVUs(t *testing.T) {
	t.Parallel()
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := vu.NewFactory(httpclient.DefaultConfig(), nil)
	plan := Plan{
		Name:    "cv",
		Flow:    vu.Flow{Scenario: "cv", Steps: []vu.Step{{Name: "hit", Method: "GET", URLTmpl: srv.URL, Check: checks.DefaultSpec()}}},
		Cfg:     VUConfig{Type: "constant-vus", VUs: 4, Duration: 600 * time.Millisecond},
		Factory: f,
	}
	sink := metrics.NewSink(1000)
	if err := RunPlans(context.Background(), []Plan{plan}, sink); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt64(&hits) == 0 {
		t.Fatal("constant-vus made no requests")
	}
	snap := sink.Snapshot()
	if len(snap) != 1 || snap[0].Aggregate == nil || snap[0].Aggregate.Count == 0 {
		t.Fatalf("expected recorded samples, got %+v", snap)
	}
}

func TestRunConstantRPS_SubSecondAndFractional(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		dur  time.Duration
		want int64
	}{
		{"sub-second", 900 * time.Millisecond, 9},
		{"fractional", 1500 * time.Millisecond, 15},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var hits int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt64(&hits, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			f := vu.NewFactory(httpclient.DefaultConfig(), nil)
			plan := Plan{
				Name:    "rps",
				Flow:    vu.Flow{Scenario: "rps", Steps: []vu.Step{{Name: "hit", Method: "GET", URLTmpl: srv.URL, Check: checks.DefaultSpec()}}},
				Cfg:     VUConfig{Type: "constant-rps", RPS: 10, Duration: c.dur},
				Factory: f,
			}
			sink := metrics.NewSink(1000)
			if err := RunPlans(context.Background(), []Plan{plan}, sink); err != nil {
				t.Fatal(err)
			}
			got := atomic.LoadInt64(&hits)
			if got < c.want-1 || got > c.want+1 {
				t.Fatalf("%s@10rps issued %d requests, want ~%d", c.dur, got, c.want)
			}
		})
	}
}

func TestRampingVUs_MaxVUsEnforced(t *testing.T) {
	t.Parallel()
	var active int64
	var peak int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := atomic.AddInt64(&active, 1)
		for {
			p := atomic.LoadInt64(&peak)
			if cur <= p || atomic.CompareAndSwapInt64(&peak, p, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt64(&active, -1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := vu.NewFactory(httpclient.DefaultConfig(), nil)
	plan := Plan{
		Name:    "ramp",
		Flow:    vu.Flow{Scenario: "ramp", Steps: []vu.Step{{Name: "hit", Method: "GET", URLTmpl: srv.URL, Check: checks.DefaultSpec()}}},
		Cfg:     VUConfig{Type: "ramping-vus", MaxVUs: 100, Stages: []Stage{{Target: 500, Duration: 600 * time.Millisecond}}, GracefulStop: 100 * time.Millisecond},
		Factory: f,
	}
	sink := metrics.NewSink(100000)
	if err := RunPlans(context.Background(), []Plan{plan}, sink); err != nil {
		t.Fatal(err)
	}
	if p := atomic.LoadInt64(&peak); p > 100 {
		t.Fatalf("MaxVUs not enforced: peak concurrent in-flight %d exceeds 100", p)
	}
}

func TestLoopVU_OnceOnlyFlowDoesNotSpin(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var iters int64
	loopIterHook = func() { atomic.AddInt64(&iters, 1) }
	defer func() { loopIterHook = nil }()

	f := vu.NewFactory(httpclient.DefaultConfig(), nil)
	plan := Plan{
		Name:    "once-only",
		Flow:    vu.Flow{Scenario: "once-only", Steps: []vu.Step{{Name: "setup", Method: "GET", URLTmpl: srv.URL, Once: true, Check: checks.DefaultSpec()}}},
		Cfg:     VUConfig{Type: "constant-vus", VUs: 1, Duration: 400 * time.Millisecond},
		Factory: f,
	}
	sink := metrics.NewSink(100000)
	if err := RunPlans(context.Background(), []Plan{plan}, sink); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Fatalf("once-only flow should issue exactly 1 request; got %d", got)
	}
	if got := atomic.LoadInt64(&iters); got > 50 {
		t.Fatalf("loopVU busy-spun: ran %d iterations in 400ms for a once-only flow (loop is not select-gated)", got)
	}
}

func TestRampingVUsConcurrentScenarios(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := vu.NewFactory(httpclient.DefaultConfig(), nil)
	mkPlan := func(name string) Plan {
		return Plan{
			Name:    name,
			Flow:    vu.Flow{Scenario: name, Steps: []vu.Step{{Name: "hit", Method: "GET", URLTmpl: srv.URL, Check: checks.DefaultSpec()}}},
			Cfg:     VUConfig{Type: "ramping-vus", Stages: []Stage{{Target: 3, Duration: 300 * time.Millisecond}, {Target: 0, Duration: 200 * time.Millisecond}}, GracefulStop: 200 * time.Millisecond},
			Factory: f,
		}
	}
	sink := metrics.NewSink(1000)
	if err := RunPlans(context.Background(), []Plan{mkPlan("a"), mkPlan("b")}, sink); err != nil {
		t.Fatal(err)
	}
	snap := sink.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("expected 2 independent scenarios, got %d", len(snap))
	}
}
