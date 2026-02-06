package scraper

import (
	"context"
	"sync"
	"time"
)

type Task func(ctx context.Context) error

type Result struct {
	Err error
}

type WorkerPool struct {
	workers int
	tasks   chan Task
	wg      sync.WaitGroup
	mu      sync.RWMutex
	rate    <-chan time.Time
	ticker  *time.Ticker
}

func NewWorkerPool(workers, buffer int) *WorkerPool {
	if workers <= 0 {
		workers = 1
	}
	if buffer < 0 {
		buffer = 0
	}
	return &WorkerPool{
		workers: workers,
		tasks:   make(chan Task, buffer),
	}
}

func (p *WorkerPool) SetRateLimit(rps int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.ticker != nil {
		p.ticker.Stop()
		p.ticker = nil
		p.rate = nil
	}
	p.mu.Unlock()
	if rps <= 0 {
		return
	}
	interval := time.Second / time.Duration(rps)
	t := time.NewTicker(interval)
	p.mu.Lock()
	p.ticker = t
	p.rate = t.C
	p.mu.Unlock()
}

func (p *WorkerPool) Submit(t Task) {
	if p == nil || t == nil {
		return
	}
	p.tasks <- t
}

func (p *WorkerPool) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.ticker != nil {
		p.ticker.Stop()
		p.ticker = nil
		p.rate = nil
	}
	p.mu.Unlock()
	close(p.tasks)
}

func (p *WorkerPool) Run(ctx context.Context) <-chan Result {
	buf := p.workers * 1024
	if buf < 1 {
		buf = 1
	}
	out := make(chan Result, buf)
	if p == nil {
		close(out)
		return out
	}

	p.wg.Add(p.workers)
	for i := 0; i < p.workers; i++ {
		go func() {
			defer p.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case t, ok := <-p.tasks:
					if !ok {
						return
					}
					if t == nil {
						continue
					}
					p.mu.RLock()
					rate := p.rate
					p.mu.RUnlock()
					if rate != nil {
						select {
						case <-ctx.Done():
							return
						case <-rate:
						}
					}
					err := t(ctx)
					select {
					case <-ctx.Done():
						return
					case out <- Result{Err: err}:
					}
				}
			}
		}()
	}

	go func() {
		p.wg.Wait()
		close(out)
	}()

	return out
}
