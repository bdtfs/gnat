package executor

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/bdtfs/gnat/internal/metrics"
	"github.com/bdtfs/gnat/internal/vu"
)

type Stage struct {
	Target   int
	Duration time.Duration
}

type VUConfig struct {
	Type         string
	Stages       []Stage
	VUs          int
	RPS          int
	StartVUs     int
	MaxVUs       int
	Duration     time.Duration
	GracefulStop time.Duration
}

type Plan struct {
	Name     string
	Flow     vu.Flow
	Identity map[string]string
	Cfg      VUConfig
	Weight   int
	Factory  *vu.Factory
}

func RunPlans(ctx context.Context, plans []Plan, sink metrics.Sink) error {
	var wg sync.WaitGroup
	for _, p := range plans {
		wg.Add(1)
		go func(p Plan) {
			defer wg.Done()
			runPlan(ctx, p, sink)
		}(p)
	}
	wg.Wait()
	return nil
}

func runPlan(ctx context.Context, p Plan, sink metrics.Sink) {
	switch p.Cfg.Type {
	case "constant-rps":
		runConstantRPS(ctx, p, sink)
	case "ramping-vus":
		runRampingVUs(ctx, p, sink)
	default:
		runConstantVUs(ctx, p, sink)
	}
}

func runConstantVUs(ctx context.Context, p Plan, sink metrics.Sink) {
	dur := p.Cfg.Duration
	if dur <= 0 {
		return
	}
	runCtx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()

	n := p.Cfg.VUs
	if n <= 0 {
		n = 1
	}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loopVU(runCtx, p, sink)
		}()
	}
	wg.Wait()
}

type vuPool struct {
	mu      sync.Mutex
	cancels []context.CancelFunc
	wg      sync.WaitGroup
	runCtx  context.Context
	p       Plan
	sink    metrics.Sink
}

func (pool *vuPool) spawn() {
	vctx, vcancel := context.WithCancel(pool.runCtx)
	pool.mu.Lock()
	pool.cancels = append(pool.cancels, vcancel)
	pool.mu.Unlock()
	pool.wg.Add(1)
	go func() {
		defer pool.wg.Done()
		loopVU(vctx, pool.p, pool.sink)
	}()
}

func (pool *vuPool) stop() {
	pool.mu.Lock()
	if len(pool.cancels) > 0 {
		c := pool.cancels[len(pool.cancels)-1]
		pool.cancels = pool.cancels[:len(pool.cancels)-1]
		pool.mu.Unlock()
		c()
		return
	}
	pool.mu.Unlock()
}

func (pool *vuPool) count() int {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return len(pool.cancels)
}

func (pool *vuPool) adjust(want int) {
	have := pool.count()
	for have < want {
		pool.spawn()
		have++
	}
	for have > want {
		pool.stop()
		have--
	}
}

func (pool *vuPool) drain() {
	pool.mu.Lock()
	for _, c := range pool.cancels {
		c()
	}
	pool.cancels = nil
	pool.mu.Unlock()
}

func runRampingVUs(ctx context.Context, p Plan, sink metrics.Sink) {
	total := time.Duration(0)
	for _, s := range p.Cfg.Stages {
		total += s.Duration
	}
	if total <= 0 {
		return
	}
	graceful := p.Cfg.GracefulStop
	runCtx, cancel := context.WithTimeout(ctx, total+graceful)
	defer cancel()

	pool := &vuPool{runCtx: runCtx, p: p, sink: sink}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	start := time.Now()
	for {
		elapsed := time.Since(start)
		want := vuCountAt(p.Cfg, elapsed)
		if p.Cfg.MaxVUs > 0 && want > p.Cfg.MaxVUs {
			want = p.Cfg.MaxVUs
		}
		pool.adjust(want)
		if elapsed >= total {
			break
		}
		select {
		case <-runCtx.Done():
			goto drain
		case <-ticker.C:
		}
	}
drain:
	pool.drain()
	cancel()
	pool.wg.Wait()
}

func runConstantRPS(ctx context.Context, p Plan, sink metrics.Sink) {
	dur := p.Cfg.Duration
	rps := p.Cfg.RPS
	if dur <= 0 || rps <= 0 {
		return
	}
	runCtx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()

	total := int(math.Round(float64(rps) * dur.Seconds()))
	if total <= 0 {
		return
	}
	interval := time.Second / time.Duration(rps)
	inflight := rps * 2
	if inflight < 16 {
		inflight = 16
	}
	sem := make(chan struct{}, inflight)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < total; i++ {
		if runCtx.Err() != nil {
			break
		}
		next := start.Add(time.Duration(i) * interval)
		if d := time.Until(next); d > 0 {
			time.Sleep(d)
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			v, err := p.Factory.New(p.Identity)
			if err != nil {
				return
			}
			v.RunIteration(runCtx, p.Flow, sink)
		}()
	}
	wg.Wait()
}

var loopIterHook func()

func loopVU(ctx context.Context, p Plan, sink metrics.Sink) {
	v, err := p.Factory.New(p.Identity)
	if err != nil {
		return
	}
	for {
		if ctx.Err() != nil {
			return
		}
		if loopIterHook != nil {
			loopIterHook()
		}
		r := v.RunIteration(ctx, p.Flow, sink)
		if len(r.Steps) == 0 {
			timer := time.NewTimer(50 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func vuCountAt(cfg VUConfig, elapsed time.Duration) int {
	prev := cfg.StartVUs
	acc := time.Duration(0)
	for _, s := range cfg.Stages {
		if elapsed <= acc+s.Duration {
			if s.Duration <= 0 {
				return s.Target
			}
			frac := float64(elapsed-acc) / float64(s.Duration)
			return prev + int(math.Round(frac*float64(s.Target-prev)))
		}
		acc += s.Duration
		prev = s.Target
	}
	return prev
}
