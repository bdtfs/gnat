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
