package stats

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Result struct {
	StatusCode int
	Latency    time.Duration
	BytesRead  int64
	Error      error
	Timestamp  time.Time
}

type Stats struct {
	TotalRequests   uint64
	SuccessRequests uint64
	FailedRequests  uint64
	TotalLatency    time.Duration
	mu              sync.Mutex
}

func New() *Stats {
	return &Stats{}
}

func (s *Stats) Record(r *Result) {
	atomic.AddUint64(&s.TotalRequests, 1)

	if r.Error != nil {
		atomic.AddUint64(&s.FailedRequests, 1)
		return
	}

	if r.StatusCode >= 200 && r.StatusCode < 400 {
		atomic.AddUint64(&s.SuccessRequests, 1)
	} else {
		atomic.AddUint64(&s.FailedRequests, 1)
	}

	s.mu.Lock()
	s.TotalLatency += r.Latency
	s.mu.Unlock()
}

func (s *Stats) Print() {
	total := atomic.LoadUint64(&s.TotalRequests)
	success := atomic.LoadUint64(&s.SuccessRequests)
	failed := atomic.LoadUint64(&s.FailedRequests)

	fmt.Printf("\nResults:\n")
	fmt.Printf("Total Requests:   %d\n", total)
	fmt.Printf("Success:          %d\n", success)
	fmt.Printf("Failed:           %d\n", failed)

	if total > 0 {
		s.mu.Lock()
		avgLatency := s.TotalLatency / time.Duration(total)
		s.mu.Unlock()
		fmt.Printf("Avg Latency:      %v\n", avgLatency)
		fmt.Printf("Success Rate:     %.2f%%\n", float64(success)/float64(total)*100)
	}
}

func (s *Stats) Total() uint64 {
	return atomic.LoadUint64(&s.TotalRequests)
}

func (s *Stats) Success() uint64 {
	return atomic.LoadUint64(&s.SuccessRequests)
}

func (s *Stats) Failed() uint64 {
	return atomic.LoadUint64(&s.FailedRequests)
}

func (s *Stats) AvgLatency() time.Duration {
	total := s.Total()
	if total == 0 {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.TotalLatency / time.Duration(total)
}
