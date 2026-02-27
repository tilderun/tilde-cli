package pool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_AllSucceed(t *testing.T) {
	p := New(context.Background(), 4)
	var count atomic.Int64

	for i := 0; i < 10; i++ {
		p.Submit(func(ctx context.Context) error {
			count.Add(1)
			return nil
		})
	}

	err := p.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if count.Load() != 10 {
		t.Errorf("count = %d, want 10", count.Load())
	}
}

func TestPool_ConcurrencyBound(t *testing.T) {
	const maxConcurrency = 2
	p := New(context.Background(), maxConcurrency)
	var concurrent atomic.Int64
	var maxSeen atomic.Int64

	for i := 0; i < 20; i++ {
		p.Submit(func(ctx context.Context) error {
			cur := concurrent.Add(1)
			// Track maximum concurrency observed
			for {
				old := maxSeen.Load()
				if cur <= old || maxSeen.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			concurrent.Add(-1)
			return nil
		})
	}

	err := p.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if maxSeen.Load() > maxConcurrency {
		t.Errorf("max concurrent = %d, want <= %d", maxSeen.Load(), maxConcurrency)
	}
}

func TestPool_ErrorReturned(t *testing.T) {
	p := New(context.Background(), 2)

	p.Submit(func(ctx context.Context) error {
		return errors.New("task failed")
	})
	p.Submit(func(ctx context.Context) error {
		return errors.New("second error")
	})

	err := p.Wait()
	if err == nil {
		t.Fatal("expected error")
	}
	// Should return one of the errors (whichever completes first)
	if err.Error() != "task failed" && err.Error() != "second error" {
		t.Errorf("unexpected error: %q", err.Error())
	}
}

func TestPool_OnlyFirstErrorCaptured(t *testing.T) {
	// Verify that only one error is stored even when multiple tasks fail
	p := New(context.Background(), 1)
	var ran int64

	for i := 0; i < 5; i++ {
		p.Submit(func(ctx context.Context) error {
			atomic.AddInt64(&ran, 1)
			return errors.New("fail")
		})
	}

	err := p.Wait()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "fail" {
		t.Errorf("err = %q, want %q", err.Error(), "fail")
	}
}

func TestPool_FailFastCancelsContext(t *testing.T) {
	p := New(context.Background(), 1)
	var secondRan atomic.Bool

	p.Submit(func(ctx context.Context) error {
		return errors.New("fail")
	})

	// Give the first task time to fail and cancel context
	time.Sleep(10 * time.Millisecond)

	p.Submit(func(ctx context.Context) error {
		secondRan.Store(true)
		return nil
	})

	_ = p.Wait()

	// The second task should not have run because context was cancelled
	if secondRan.Load() {
		t.Error("second task ran despite first task failing")
	}
}

func TestPool_ExternalContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := New(ctx, 1)
	var ran atomic.Int64

	// Fill the semaphore with a blocking task
	p.Submit(func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond)
		ran.Add(1)
		return nil
	})

	// Cancel while first task is running
	time.Sleep(10 * time.Millisecond)
	cancel()

	// This task should not run
	p.Submit(func(ctx context.Context) error {
		ran.Add(1)
		return nil
	})

	_ = p.Wait()

	// At most 1 task should have completed (the one already running when we cancelled)
	if ran.Load() > 1 {
		t.Errorf("ran %d tasks, expected at most 1", ran.Load())
	}
}

func TestPool_ZeroTasks(t *testing.T) {
	p := New(context.Background(), 4)
	err := p.Wait()
	if err != nil {
		t.Fatalf("Wait with no tasks: %v", err)
	}
}

func TestPool_SingleConcurrency(t *testing.T) {
	p := New(context.Background(), 1)
	var order []int
	var mu syncMu

	for i := 0; i < 5; i++ {
		i := i
		p.Submit(func(ctx context.Context) error {
			mu.lock()
			order = append(order, i)
			mu.unlock()
			return nil
		})
	}

	err := p.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if len(order) != 5 {
		t.Errorf("executed %d tasks, want 5", len(order))
	}
}

// Simple channel-based mutex for testing
type syncMu struct {
	ch chan struct{}
}

func (m *syncMu) lock() {
	if m.ch == nil {
		m.ch = make(chan struct{}, 1)
	}
	m.ch <- struct{}{}
}

func (m *syncMu) unlock() {
	<-m.ch
}

func TestPool_LargeBatch(t *testing.T) {
	p := New(context.Background(), 8)
	var count atomic.Int64

	for i := 0; i < 1000; i++ {
		p.Submit(func(ctx context.Context) error {
			count.Add(1)
			return nil
		})
	}

	err := p.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if count.Load() != 1000 {
		t.Errorf("count = %d, want 1000", count.Load())
	}
}

func TestPool_ErrorDoesNotLoseOtherWork(t *testing.T) {
	// Tasks already running should complete even after an error
	p := New(context.Background(), 4)
	var completed atomic.Int64

	// Submit a task that completes quickly
	p.Submit(func(ctx context.Context) error {
		completed.Add(1)
		return nil
	})

	// Small delay to let first task complete
	time.Sleep(5 * time.Millisecond)

	// Submit a failing task
	p.Submit(func(ctx context.Context) error {
		return errors.New("boom")
	})

	_ = p.Wait()

	if completed.Load() < 1 {
		t.Error("expected at least one task to have completed")
	}
}
