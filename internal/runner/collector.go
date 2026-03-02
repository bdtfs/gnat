package runner

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bdtfs/gnat/internal/models"
)

type Collector struct {
	mu   sync.RWMutex
	runs map[string]*models.Stats
}

func NewCollector() *Collector {
	return &Collector{
		runs: make(map[string]*models.Stats),
	}
}

func (c *Collector) StartRunStatsProcessing(run *models.Run) chan<- *Result {
	stats := NewStats()
	run.Stats = stats

	c.mu.Lock()
	c.runs[run.ID] = stats
	c.mu.Unlock()

	ch := make(chan *Result, 100)
	done := make(chan struct{})

	go func() {
		for r := range ch {
			c.ProcessOneResult(stats, r)
		}
		close(done)
	}()

	go c.captureTimeSeries(stats, done)

	return ch
}

// captureTimeSeries snapshots stats every second until done is closed.
func (c *Collector) captureTimeSeries(s *models.Stats, done <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var prevSuccess, prevFailed uint64

	for {
		select {
		case <-done:
			// Capture one final point.
			c.snapshotTimeSeries(s, &prevSuccess, &prevFailed)
			return
		case <-ticker.C:
			c.snapshotTimeSeries(s, &prevSuccess, &prevFailed)
		}
	}
}

func (c *Collector) snapshotTimeSeries(s *models.Stats, prevSuccess, prevFailed *uint64) {
	nowMs := time.Since(s.StartedAt).Milliseconds()

	curSuccess := atomic.LoadUint64(&s.SuccessRequests)
	curFailed := atomic.LoadUint64(&s.FailedRequests)

	intervalSuccess := int64(curSuccess - *prevSuccess)
	intervalFailed := int64(curFailed - *prevFailed)
	intervalTotal := intervalSuccess + intervalFailed

	*prevSuccess = curSuccess
	*prevFailed = curFailed

	var errorRate float64
	if intervalTotal > 0 {
		errorRate = float64(intervalFailed) / float64(intervalTotal)
	}

	rps := float64(intervalTotal) // Per-second since ticker fires every second.

	// Compute latency percentiles from the full latency slice.
	s.LatenciesMu.Lock()
	lat := make([]time.Duration, len(s.Latencies))
	copy(lat, s.Latencies)
	s.LatenciesMu.Unlock()

	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })

	point := models.TimeSeriesPoint{
		TimestampMs:  nowMs,
		P50Latency:   tsPercentile(lat, 0.50),
		P90Latency:   tsPercentile(lat, 0.90),
		P95Latency:   tsPercentile(lat, 0.95),
		P99Latency:   tsPercentile(lat, 0.99),
		RPS:          rps,
		ErrorRate:    errorRate,
		SuccessCount: intervalSuccess,
		FailedCount:  intervalFailed,
	}

	s.TimeSeriesMu.Lock()
	s.TimeSeries = append(s.TimeSeries, point)
	s.TimeSeriesMu.Unlock()
}

// tsPercentile computes a percentile from a sorted duration slice.
func tsPercentile(sorted []time.Duration, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return float64(sorted[idx].Milliseconds())
}

func (c *Collector) ProcessOneResult(s *models.Stats, r *Result) {
	atomic.AddUint64(&s.TotalRequests, 1)

	if r.Error != nil {
		atomic.AddUint64(&s.FailedRequests, 1)
		s.ErrorsMu.Lock()
		s.Errors = append(s.Errors, r.Error.Error())
		s.ErrorsMu.Unlock()
		return
	}

	atomic.AddUint64(&s.TotalBytesRead, uint64(r.BytesRead))

	s.StatusMu.Lock()
	ptr, ok := s.StatusCodes[r.StatusCode]
	if !ok {
		var n uint64
		ptr = &n
		s.StatusCodes[r.StatusCode] = ptr
	}
	s.StatusMu.Unlock()
	atomic.AddUint64(ptr, 1)

	if r.StatusCode >= 200 && r.StatusCode < 400 {
		atomic.AddUint64(&s.SuccessRequests, 1)
	} else {
		atomic.AddUint64(&s.FailedRequests, 1)
	}

	s.LatenciesMu.Lock()
	s.Latencies = append(s.Latencies, r.Latency)
	s.LatenciesMu.Unlock()

	s.LatencyMu.Lock()
	s.TotalLatency += r.Latency
	s.LatencyMu.Unlock()
}

func (c *Collector) GetStats(runID string) *models.Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.runs[runID]
}

func (c *Collector) DeleteRun(runID string) {
	c.mu.Lock()
	delete(c.runs, runID)
	c.mu.Unlock()
}
