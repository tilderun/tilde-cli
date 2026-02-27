package pool

import (
	"context"
	"sync"
)

// Pool is a bounded concurrency worker pool with fail-fast semantics.
type Pool struct {
	sem    chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
	err    error
	mu     sync.Mutex
}

// New creates a worker pool with the given concurrency limit.
func New(ctx context.Context, concurrency int) *Pool {
	ctx, cancel := context.WithCancel(ctx)
	return &Pool{
		sem:    make(chan struct{}, concurrency),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Submit enqueues a task. It blocks if the pool is at capacity.
// If the pool's context is cancelled (due to a prior error or external cancellation),
// Submit returns immediately without running fn.
func (p *Pool) Submit(fn func(ctx context.Context) error) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		// Acquire semaphore or bail on cancellation
		select {
		case p.sem <- struct{}{}:
		case <-p.ctx.Done():
			return
		}
		defer func() { <-p.sem }()

		// Check cancellation again after acquiring
		if p.ctx.Err() != nil {
			return
		}

		if err := fn(p.ctx); err != nil {
			p.once.Do(func() {
				p.mu.Lock()
				p.err = err
				p.mu.Unlock()
				p.cancel()
			})
		}
	}()
}

// Wait blocks until all submitted tasks complete and returns the first error (if any).
func (p *Pool) Wait() error {
	p.wg.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}
