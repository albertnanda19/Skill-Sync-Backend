package scraper

import (
	"context"
	"sync"
)

type Task func(ctx context.Context) error

type Result struct {
	Err error
}

type WorkerPool struct {
	workers int
	tasks   chan Task
	wg      sync.WaitGroup
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
	close(p.tasks)
}

func (p *WorkerPool) Run(ctx context.Context) <-chan Result {
	out := make(chan Result)
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
