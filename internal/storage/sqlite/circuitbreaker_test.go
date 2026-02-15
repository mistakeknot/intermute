package sqlite

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestBreakerStartsClosed(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second)
	if cb.State() != StateClosed {
		t.Fatalf("expected closed, got %s", cb.State())
	}
}

func TestBreakerOpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second)
	testErr := errors.New("fail")

	for i := 0; i < 5; i++ {
		_ = cb.Execute(func() error { return testErr })
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected open after %d failures, got %s", 5, cb.State())
	}
}

func TestBreakerRejectsWhenOpen(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second)
	testErr := errors.New("fail")

	for i := 0; i < 5; i++ {
		_ = cb.Execute(func() error { return testErr })
	}

	called := false
	err := cb.Execute(func() error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if called {
		t.Fatal("fn should not have been called when breaker is open")
	}
}

func TestBreakerResetsAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(5, 100*time.Millisecond)
	now := time.Now()
	cb.nowFunc = func() time.Time { return now }
	testErr := errors.New("fail")

	// Trip the breaker
	for i := 0; i < 5; i++ {
		_ = cb.Execute(func() error { return testErr })
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected open, got %s", cb.State())
	}

	// Advance time past reset timeout
	now = now.Add(200 * time.Millisecond)

	// Next call should be the probe (half-open transition)
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("expected probe to succeed, got %v", err)
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected closed after successful probe, got %s", cb.State())
	}
}

func TestBreakerProbeFailureReOpens(t *testing.T) {
	cb := NewCircuitBreaker(5, 100*time.Millisecond)
	now := time.Now()
	cb.nowFunc = func() time.Time { return now }
	testErr := errors.New("fail")

	for i := 0; i < 5; i++ {
		_ = cb.Execute(func() error { return testErr })
	}

	now = now.Add(200 * time.Millisecond)
	_ = cb.Execute(func() error { return testErr })
	if cb.State() != StateOpen {
		t.Fatalf("expected open after probe failure, got %s", cb.State())
	}
}

func TestBreakerSuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second)
	testErr := errors.New("fail")

	// 3 failures
	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return testErr })
	}
	// 1 success resets count
	_ = cb.Execute(func() error { return nil })
	// 3 more failures — should NOT open (only 3 consecutive, not 5)
	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return testErr })
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected closed (3+3 non-consecutive < threshold 5), got %s", cb.State())
	}
}

func TestBreakerConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(100, 30*time.Second) // high threshold to avoid tripping
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cb.Execute(func() error { return nil })
			_ = cb.State()
		}()
	}
	wg.Wait()
}
