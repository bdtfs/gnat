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

	var (
		mu      sync.Mutex
		cancels []context.CancelFunc
		wg      sync.WaitGroup
	)
	spawn := func() {
		vctx, vcancel := context.WithCancel(runCtx)
		mu.Lock()
		cancels = append(cancels, vcancel)
		mu.Unlock()
		wg.Add(1)
		go func() {
			defer wg.Done()
			loopVU(vctx, p, sink)
		}()
	}
	stop := func() {
		mu.Lock()
		if len(cancels) > 0 {
			c := cancels[len(cancels)-1]
			cancels = cancels[:len(cancels)-1]
			mu.Unlock()
			c()
			return
		}
		mu.Unlock()
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	start := time.Now()
	for {
		elapsed := time.Since(start)
		want := vuCountAt(p.Cfg, elapsed)
		mu.Lock()
		have := len(cancels)
		mu.Unlock()
		for have < want {
			spawn()
			have++
		}
		for have > want {
			stop()
			have--
		}
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
	mu.Lock()
	for _, c := range cancels {
		c()
	}
	cancels = nil
	mu.Unlock()
	cancel()
	wg.Wait()
}

func runConstantRPS(ctx context.Context, p Plan, sink metrics.Sink) {
	dur := p.Cfg.Duration
	rps := p.Cfg.RPS
	if dur <= 0 || rps <= 0 {
		return
	}
	runCtx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()

	total := rps * int(dur/time.Second)
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
			v.RunIteration(runCtx, p.Flow, sink, true)
		}()
	}
	wg.Wait()
}

func loopVU(ctx context.Context, p Plan, sink metrics.Sink) {
	v, err := p.Factory.New(p.Identity)
	if err != nil {
		return
	}
	first := true
	for {
		if ctx.Err() != nil {
			return
		}
		r := v.RunIteration(ctx, p.Flow, sink, first)
		if r.Completed {
			first = false
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
