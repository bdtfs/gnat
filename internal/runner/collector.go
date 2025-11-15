package runner

import (
	"sync"
	"sync/atomic"

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

	go func() {
		for r := range ch {
			c.ProcessOneResult(stats, r)
		}
	}()

	return ch
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
