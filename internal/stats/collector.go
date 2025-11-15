package stats

import (
	"context"
	"sync"
)

type Collector struct {
	runs   map[string]*Stats
	mu     sync.RWMutex
	buffer map[string]chan *Result
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewCollector() *Collector {
	ctx, cancel := context.WithCancel(context.Background())
	return &Collector{
		runs:   make(map[string]*Stats),
		buffer: make(map[string]chan *Result),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (c *Collector) StartRun(runID string) chan<- *Result {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats := New()
	c.runs[runID] = stats

	resultChan := make(chan *Result, 1000)
	c.buffer[runID] = resultChan

	c.wg.Add(1)
	go c.worker(runID, stats, resultChan)

	return resultChan
}

func (c *Collector) worker(runID string, stats *Stats, results <-chan *Result) {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case result, ok := <-results:
			if !ok {
				return
			}
			stats.Record(result)
		}
	}
}

func (c *Collector) GetStats(runID string) *Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.runs[runID]
}

func (c *Collector) StopRun(runID string) {
	c.mu.Lock()
	if ch, exists := c.buffer[runID]; exists {
		close(ch)
		delete(c.buffer, runID)
	}
	c.mu.Unlock()
}

func (c *Collector) Shutdown() {
	c.cancel()
	c.mu.Lock()
	for _, ch := range c.buffer {
		close(ch)
	}
	c.buffer = make(map[string]chan *Result)
	c.mu.Unlock()
	c.wg.Wait()
}
