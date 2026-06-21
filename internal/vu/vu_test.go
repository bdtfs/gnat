package vu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bdtfs/gnat/internal/checks"
	"github.com/bdtfs/gnat/internal/extract"
	"github.com/bdtfs/gnat/internal/metrics"
	httpclient "github.com/bdtfs/gnat/pkg/clients/http"
)

func testFactory(t *testing.T, globals map[string]string) *Factory {
	t.Helper()
	return NewFactory(httpclient.DefaultConfig(), globals)
}

func TestVU_CookieAndVarIsolation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("sess"); err != nil {
			http.SetCookie(w, &http.Cookie{Name: "sess", Value: "x", Path: "/"})
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := testFactory(t, nil)
	v1, err := f.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := f.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, _, _, _, _, err := v1.Do(context.Background(), http.MethodGet, srv.URL, "", nil, 0); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	if got := v1.Jar.Cookies(req.URL); len(got) != 1 {
		t.Errorf("v1 should have 1 cookie, got %d", len(got))
	}
	if got := v2.Jar.Cookies(req.URL); len(got) != 0 {
		t.Errorf("v2 jar must be isolated from v1, got %d cookies", len(got))
	}

	v1.Vars["k"] = "v1val"
	if _, ok := v2.Vars["k"]; ok {
		t.Error("v2 vars must be isolated from v1")
	}
}

func TestVU_ExtractionChaining(t *testing.T) {
	t.Parallel()

	var detailPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/list":
			w.Write([]byte(`{"shows":[{"slug":"spongebob"}]}`))
		case strings.HasPrefix(r.URL.Path, "/detail/"):
			detailPath = r.URL.Path
			w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()

	f := testFactory(t, map[string]string{"base": srv.URL})
	v, err := f.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	flow := Flow{
		Scenario: "chain",
		Steps: []Step{
			{
				Name: "list", Method: "GET", URLTmpl: "{{base}}/list",
				Check:   checks.DefaultSpec(),
				Extract: []extract.Spec{{Var: "slug", Source: "json", Path: "shows[0].slug", Required: true}},
			},
			{Name: "detail", Method: "GET", URLTmpl: "{{base}}/detail/{{slug}}", Check: checks.DefaultSpec()},
		},
	}
	sink := metrics.NewSink(100)
	res := v.RunIteration(context.Background(), flow, sink)
	if !res.Completed {
		t.Fatalf("iteration should complete, got %+v", res)
	}
	if detailPath != "/detail/spongebob" {
		t.Errorf("extracted slug not interpolated into next URL: got %q", detailPath)
	}
}

func TestVU_RangeAndTTFB(t *testing.T) {
	t.Parallel()

	big := strings.Repeat("a", 1<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(big))
	}))
	defer srv.Close()

	f := testFactory(t, nil)
	v, _ := f.New(nil)
	status, _, _, body, ttfb, dur, err := v.Do(context.Background(), http.MethodGet, srv.URL, "", nil, 65536)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK {
		t.Fatalf("status %d", status)
	}
	if len(body) != 65536 {
		t.Errorf("read_bytes_cap not honored: got %d bytes", len(body))
	}
	if ttfb <= 0 || dur <= 0 {
		t.Errorf("ttfb/dur must be positive: ttfb=%f dur=%f", ttfb, dur)
	}
}

func TestVU_RequiredCheckAborts(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := testFactory(t, nil)
	v, _ := f.New(nil)
	flow := Flow{
		Scenario: "abort",
		Steps: []Step{
			{Name: "boom", Method: "GET", URLTmpl: srv.URL, Check: checks.Spec{ExpectStatus: []int{200}, Required: true}},
			{Name: "never", Method: "GET", URLTmpl: srv.URL, Check: checks.DefaultSpec()},
		},
	}
	res := v.RunIteration(context.Background(), Flow(flow), metrics.NewSink(100))
	if res.Completed {
		t.Error("required failure must abort the iteration")
	}
	if len(res.Steps) != 1 {
		t.Errorf("should stop after the aborting step, ran %d steps", len(res.Steps))
	}
}

func TestVU_OnceStepNotRerunAfterAbort(t *testing.T) {
	t.Parallel()

	var onceHits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/once":
			atomic.AddInt64(&onceHits, 1)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	f := testFactory(t, map[string]string{"base": srv.URL})
	v, err := f.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	flow := Flow{
		Scenario: "once",
		Steps: []Step{
			{Name: "login", Method: "GET", URLTmpl: "{{base}}/once", Once: true, Check: checks.DefaultSpec()},
			{Name: "work", Method: "GET", URLTmpl: "{{base}}/work", Check: checks.Spec{ExpectStatus: []int{200}, Required: true}},
		},
	}
	sink := metrics.NewSink(100)
	r1 := v.RunIteration(context.Background(), flow, sink)
	if r1.Completed {
		t.Fatal("iter 1 should abort on the required failing step")
	}
	v.RunIteration(context.Background(), flow, sink)

	if got := atomic.LoadInt64(&onceHits); got != 1 {
		t.Errorf("once-step must run exactly once across iterations, ran %d times", got)
	}
}

func TestInterpolate(t *testing.T) {
	t.Parallel()
	vars := map[string]string{"a": "1", "b": "two"}
	if got := Interpolate("x={{a}}&y={{b}}&z={{missing}}", vars); got != "x=1&y=two&z={{missing}}" {
		t.Errorf("got %q", got)
	}
}
