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

	client := httpclient.New()

	ch := r.collector.StartRunStatsProcessing(run)
	defer close(ch)

	ticker := time.NewTicker(time.Second / time.Duration(setup.RPS))
	defer ticker.Stop()

	stop := time.Now().Add(setup.Duration)
	var wg sync.WaitGroup

	loopCtx := context.WithoutCancel(ctx)

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil

		case <-ticker.C:
			if time.Now().After(stop) {
				wg.Wait()
				return nil
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				ch <- send(loopCtx, client, setup.Method, setup.URL, setup.Body)
			}()
		}
	}
}
