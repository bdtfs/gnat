package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bdtfs/gnat/internal/models"
	httpclient "github.com/bdtfs/gnat/pkg/clients/http"
)

func (r *Runner) runLoop(
	ctx context.Context,
	run *models.Run,
	setup *models.Setup,
) error {
	if setup.URL == "" {
		return fmt.Errorf("url cannot be empty")
	}

	if setup.RPS <= 0 {
		return fmt.Errorf("rps must be greater than 0")
	}

	total := setup.RPS * int(setup.Duration/time.Second)

	client := httpclient.New()
	ch := r.collector.StartRunStatsProcessing(run)
	defer close(ch)

	interval := time.Second / time.Duration(setup.RPS)
	start := time.Now()

	var wg sync.WaitGroup
	loopCtx := context.WithoutCancel(ctx)

	for i := 0; i < total; i++ {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		default:
		}

		now := time.Now()
		next := start.Add(time.Duration(i) * interval)

		if now.Before(next) {
			time.Sleep(next.Sub(now))
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			ch <- send(loopCtx, client, setup.Method, setup.URL, setup.Body)
		}()
	}

	wg.Wait()
	return nil
}
