package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bdtfs/gnat/internal/models"
)

func TestRunLoop_SubSecondAndFractionalCount(t *testing.T) {
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

			runner := newTestRunner()
			setup := models.NewSetup("t", "d", http.MethodGet, srv.URL, nil, nil, 10, c.dur)
			run := models.NewRun(setup.ID)

			if err := runner.runLoop(context.Background(), run, setup); err != nil {
				t.Fatal(err)
			}
			got := atomic.LoadInt64(&hits)
			if got < c.want-1 || got > c.want+1 {
				t.Fatalf("%s@10rps issued %d requests, want ~%d", c.dur, got, c.want)
			}
		})
	}
}
