package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	httpclient "github.com/bdtfs/gnat/pkg/clients/http"
)

func (r *Runner) run(
	ctx context.Context,
	runID string,
	cfg *httpclient.Config,
	method, url string,
	rps int,
	d time.Duration,
	body []byte,
) error {
	if url == "" {
		return fmt.Errorf("url cannot be empty")
	}

	if rps <= 0 {
		return fmt.Errorf("rps must be greater than 0")
	}

	var client *http.Client
	if cfg != nil {
		client = httpclient.WithConfig(cfg)
	} else {
		client = httpclient.New()
	}

	resultChan := r.collector.StartRun(runID)
	defer r.collector.StopRun(runID)

	ticker := time.NewTicker(time.Second / time.Duration(rps))
	defer ticker.Stop()

	stopTime := time.Now().Add(d)
	var wg sync.WaitGroup
	requestCtx := context.WithoutCancel(ctx)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			wg.Wait()
			return nil
		case <-ticker.C:
			if time.Now().After(stopTime) {
				ticker.Stop()
				wg.Wait()
				return nil
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				result := send(requestCtx, client, method, url, body)
				resultChan <- result
			}()
		}
	}
}

func send(ctx context.Context, client *http.Client, method, url string, body []byte) *Result {
	result := &Result{
		Timestamp: time.Now(),
	}

	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		result.Error = fmt.Errorf("create request: %w", err)
		return result
	}

	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/octet-stream")
	}

	start := time.Now()
	resp, err := client.Do(req)
	result.Latency = time.Since(start)

	if err != nil {
		result.Error = fmt.Errorf("do request: %w", err)
		return result
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			result.Error = fmt.Errorf("close response body: %w", err)
		}
	}()

	result.StatusCode = resp.StatusCode

	written, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		result.Error = fmt.Errorf("read response: %w", err)
		return result
	}
	result.BytesRead = written

	return result
}
