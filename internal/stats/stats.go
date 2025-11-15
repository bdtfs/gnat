package stats

import (
	"sort"
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
	TotalBytesRead  uint64

	statusCodes map[int]*uint64
	statusMu    sync.RWMutex

	latencies   []time.Duration
	latenciesMu sync.Mutex

	TotalLatency time.Duration
	latencyMu    sync.Mutex

	errors   []string
	errorsMu sync.RWMutex
}

func New() *Stats {
	return &Stats{
		statusCodes: make(map[int]*uint64),
		latencies:   make([]time.Duration, 0, 10000),
		errors:      make([]string, 0),
	}
}

func (s *Stats) Record(r *Result) {
	atomic.AddUint64(&s.TotalRequests, 1)

	if r.Error != nil {
		atomic.AddUint64(&s.FailedRequests, 1)
		s.errorsMu.Lock()
		s.errors = append(s.errors, r.Error.Error())
		s.errorsMu.Unlock()
		return
	}

	atomic.AddUint64(&s.TotalBytesRead, uint64(r.BytesRead))

	s.statusMu.Lock()
	if _, exists := s.statusCodes[r.StatusCode]; !exists {
		var count uint64
		s.statusCodes[r.StatusCode] = &count
	}
	statusCount := s.statusCodes[r.StatusCode]
	s.statusMu.Unlock()

	atomic.AddUint64(statusCount, 1)

	if r.StatusCode >= 200 && r.StatusCode < 400 {
		atomic.AddUint64(&s.SuccessRequests, 1)
	} else {
		atomic.AddUint64(&s.FailedRequests, 1)
	}

	s.latenciesMu.Lock()
	s.latencies = append(s.latencies, r.Latency)
	s.latenciesMu.Unlock()

	s.latencyMu.Lock()
	s.TotalLatency += r.Latency
	s.latencyMu.Unlock()
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

func (s *Stats) BytesRead() uint64 {
	return atomic.LoadUint64(&s.TotalBytesRead)
}

func (s *Stats) AvgLatency() time.Duration {
	total := s.Total()
	if total == 0 {
		return 0
	}
	s.latencyMu.Lock()
	defer s.latencyMu.Unlock()
	return s.TotalLatency / time.Duration(total)
}

func (s *Stats) MinLatency() time.Duration {
	s.latenciesMu.Lock()
	defer s.latenciesMu.Unlock()

	if len(s.latencies) == 0 {
		return 0
	}

	min := s.latencies[0]
	for _, l := range s.latencies {
		if l < min {
			min = l
		}
	}
	return min
}

func (s *Stats) MaxLatency() time.Duration {
	s.latenciesMu.Lock()
	defer s.latenciesMu.Unlock()

	if len(s.latencies) == 0 {
		return 0
	}

	max := s.latencies[0]
	for _, l := range s.latencies {
		if l > max {
			max = l
		}
	}
	return max
}

func (s *Stats) Percentile(p float64) time.Duration {
	s.latenciesMu.Lock()
	defer s.latenciesMu.Unlock()

	if len(s.latencies) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(s.latencies))
	copy(sorted, s.latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	index := int(float64(len(sorted)) * p)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

func (s *Stats) StatusCodeDistribution() map[int]uint64 {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()

	dist := make(map[int]uint64)
	for code, count := range s.statusCodes {
		dist[code] = atomic.LoadUint64(count)
	}
	return dist
}

func (s *Stats) Errors() []string {
	s.errorsMu.RLock()
	defer s.errorsMu.RUnlock()

	result := make([]string, len(s.errors))
	copy(result, s.errors)
	return result
}

func (s *Stats) RPS(duration time.Duration) float64 {
	if duration == 0 {
		return 0
	}
	return float64(s.Total()) / duration.Seconds()
}
