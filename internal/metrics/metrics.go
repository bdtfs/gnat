package metrics

import (
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/bdtfs/gnat/internal/models"
)

const defaultRingSize = 10000

var (
	sampleRand   = rand.New(rand.NewSource(time.Now().UnixNano()))
	sampleRandMu sync.Mutex
)

func sampleIndex(n int64) int64 {
	sampleRandMu.Lock()
	defer sampleRandMu.Unlock()
	return sampleRand.Int63n(n)
}

type Sample struct {
	Scenario    string
	Step        string
	Status      int
	DurationMs  float64
	TTFBMs      float64
	BytesRead   int64
	CheckPassed bool
	Err         error
}

type Percentiles struct {
	P50 float64
	P90 float64
	P95 float64
	P99 float64
}

type StepStats struct {
	Scenario         string
	Step             string
	Count            int64
	Failed           int64
	Latency          Percentiles
	TTFB             Percentiles
	BytesRead        int64
	StatusCounts     map[int]int64
	CheckFailReasons map[string]int64
	latencySamples   []float64
}

type ScenarioStats struct {
	Scenario  string
	Steps     []*StepStats
	Aggregate *StepStats
}

type Sink interface {
	Record(Sample)
	Snapshot() []ScenarioStats
}

type ring struct {
	buf   []float64
	size  int
	count int64
}

func newRing(size int) *ring {
	return &ring{buf: make([]float64, 0, size), size: size}
}

func (r *ring) add(v float64) {
	r.count++
	if len(r.buf) < r.size {
		r.buf = append(r.buf, v)
		return
	}
	j := sampleIndex(r.count)
	if j < int64(r.size) {
		r.buf[j] = v
	}
}

func (r *ring) values() []float64 {
	out := make([]float64, len(r.buf))
	copy(out, r.buf)
	return out
}

type bucket struct {
	scenario         string
	step             string
	count            int64
	failed           int64
	bytesRead        int64
	statusCounts     map[int]int64
	checkFailReasons map[string]int64
	latency          *ring
	ttfb             *ring
}

type stepKey struct {
	scenario string
	step     string
}

type sink struct {
	mu       sync.Mutex
	ringSize int
	buckets  map[stepKey]*bucket
}

func NewSink(ringSize int) Sink {
	if ringSize <= 0 {
		ringSize = defaultRingSize
	}
	return &sink{
		ringSize: ringSize,
		buckets:  make(map[stepKey]*bucket),
	}
}

func (s *sink) Record(sm Sample) {
	key := stepKey{scenario: sm.Scenario, step: sm.Step}

	s.mu.Lock()
	defer s.mu.Unlock()

	b := s.buckets[key]
	if b == nil {
		b = &bucket{
			scenario:         sm.Scenario,
			step:             sm.Step,
			statusCounts:     make(map[int]int64),
			checkFailReasons: make(map[string]int64),
			latency:          newRing(s.ringSize),
			ttfb:             newRing(s.ringSize),
		}
		s.buckets[key] = b
	}

	b.count++
	b.bytesRead += sm.BytesRead

	if sm.Err != nil || !sm.CheckPassed {
		b.failed++
	}

	if sm.Status != 0 {
		b.statusCounts[sm.Status]++
	}

	if sm.Err != nil {
		b.checkFailReasons["error"]++
	} else if !sm.CheckPassed {
		b.checkFailReasons["check-failed"]++
	}

	b.latency.add(sm.DurationMs)
	b.ttfb.add(sm.TTFBMs)
}

func (s *sink) Snapshot() []ScenarioStats {
	s.mu.Lock()
	buckets := make([]*bucket, 0, len(s.buckets))
	for _, b := range s.buckets {
		buckets = append(buckets, snapshotBucket(b))
	}
	s.mu.Unlock()

	byScenario := make(map[string][]*bucket)
	scenarioOrder := make([]string, 0)
	for _, b := range buckets {
		if _, ok := byScenario[b.scenario]; !ok {
			scenarioOrder = append(scenarioOrder, b.scenario)
		}
		byScenario[b.scenario] = append(byScenario[b.scenario], b)
	}
	sort.Strings(scenarioOrder)

	out := make([]ScenarioStats, 0, len(scenarioOrder))
	for _, scenario := range scenarioOrder {
		bs := byScenario[scenario]
		sort.Slice(bs, func(i, j int) bool { return bs[i].step < bs[j].step })

		steps := make([]*StepStats, 0, len(bs))
		agg := &bucket{
			scenario:         scenario,
			step:             "*",
			statusCounts:     make(map[int]int64),
			checkFailReasons: make(map[string]int64),
			latency:          &ring{},
			ttfb:             &ring{},
		}
		for _, b := range bs {
			steps = append(steps, stepStatsFromBucket(b))
			mergeBucket(agg, b)
		}

		out = append(out, ScenarioStats{
			Scenario:  scenario,
			Steps:     steps,
			Aggregate: stepStatsFromBucket(agg),
		})
	}

	return out
}

func snapshotBucket(b *bucket) *bucket {
	cp := &bucket{
		scenario:         b.scenario,
		step:             b.step,
		count:            b.count,
		failed:           b.failed,
		bytesRead:        b.bytesRead,
		statusCounts:     make(map[int]int64, len(b.statusCounts)),
		checkFailReasons: make(map[string]int64, len(b.checkFailReasons)),
		latency:          &ring{buf: b.latency.values()},
		ttfb:             &ring{buf: b.ttfb.values()},
	}
	for k, v := range b.statusCounts {
		cp.statusCounts[k] = v
	}
	for k, v := range b.checkFailReasons {
		cp.checkFailReasons[k] = v
	}
	return cp
}

func mergeBucket(dst, src *bucket) {
	dst.count += src.count
	dst.failed += src.failed
	dst.bytesRead += src.bytesRead
	for k, v := range src.statusCounts {
		dst.statusCounts[k] += v
	}
	for k, v := range src.checkFailReasons {
		dst.checkFailReasons[k] += v
	}
	dst.latency.buf = append(dst.latency.buf, src.latency.buf...)
	dst.ttfb.buf = append(dst.ttfb.buf, src.ttfb.buf...)
}

func stepStatsFromBucket(b *bucket) *StepStats {
	statusCounts := make(map[int]int64, len(b.statusCounts))
	for k, v := range b.statusCounts {
		statusCounts[k] = v
	}
	checkFailReasons := make(map[string]int64, len(b.checkFailReasons))
	for k, v := range b.checkFailReasons {
		checkFailReasons[k] = v
	}
	latencySamples := make([]float64, len(b.latency.buf))
	copy(latencySamples, b.latency.buf)
	return &StepStats{
		Scenario:         b.scenario,
		Step:             b.step,
		Count:            b.count,
		Failed:           b.failed,
		Latency:          percentilesOf(b.latency.buf),
		TTFB:             percentilesOf(b.ttfb.buf),
		BytesRead:        b.bytesRead,
		StatusCounts:     statusCounts,
		CheckFailReasons: checkFailReasons,
		latencySamples:   latencySamples,
	}
}

func percentilesOf(values []float64) Percentiles {
	if len(values) == 0 {
		return Percentiles{}
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	return Percentiles{
		P50: pick(sorted, 0.50),
		P90: pick(sorted, 0.90),
		P95: pick(sorted, 0.95),
		P99: pick(sorted, 0.99),
	}
}

func pick(sorted []float64, p float64) float64 {
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

func (s *StepStats) ToModelsStats() *models.Stats {
	if s == nil {
		return nil
	}

	success := s.Count - s.Failed
	if success < 0 {
		success = 0
	}

	statusCodes := make(map[int]*uint64, len(s.StatusCounts))
	for code, n := range s.StatusCounts {
		v := uint64(n)
		statusCodes[code] = &v
	}

	latencies := make([]time.Duration, len(s.latencySamples))
	for i, ms := range s.latencySamples {
		latencies[i] = time.Duration(ms * float64(time.Millisecond))
	}

	out := &models.Stats{
		TotalRequests:   uint64(s.Count),
		SuccessRequests: uint64(success),
		FailedRequests:  uint64(s.Failed),
		TotalBytesRead:  uint64(s.BytesRead),
		StatusCodes:     statusCodes,
		Latencies:       latencies,
		StatusMu:        sync.RWMutex{},
		LatenciesMu:     sync.Mutex{},
		LatencyMu:       sync.Mutex{},
		ErrorsMu:        sync.RWMutex{},
		TimeSeriesMu:    sync.RWMutex{},
		StartedAt:       time.Time{},
	}

	return out
}
