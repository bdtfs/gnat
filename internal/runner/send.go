package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Result struct {
	StatusCode int
	Latency    time.Duration
	BytesRead  int64
	Error      error
	Timestamp  time.Time
}

func send(ctx context.Context, client *http.Client, method, url string, body []byte, headers map[string]string) *Result {
	res := &Result{Timestamp: time.Now()}

	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		res.Error = fmt.Errorf("create request: %w", err)
		return res
	}

	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/octet-stream")
	}

	// Configured headers are applied last so they override defaults such as
	// the body Content-Type above and Go's automatic User-Agent.
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	res.Latency = time.Since(start)

	if err != nil {
		res.Error = fmt.Errorf("do request: %w", err)
		return res
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			res.Error = fmt.Errorf("close body: %w", err)
		}
	}()

	res.StatusCode = resp.StatusCode

	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		res.Error = fmt.Errorf("read body: %w", err)
		return res
	}

	res.BytesRead = n
	return res
}
