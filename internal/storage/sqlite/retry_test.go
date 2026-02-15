package sqlite

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRetrySucceedsOnTransientLock(t *testing.T) {
	calls := 0
	err := retryOnDBLockInternal(DefaultRetryConfig(), func() error {
		calls++
		if calls <= 3 {
			return errors.New("database is locked")
		}
		return nil
	}, func(d time.Duration) {}) // no-op sleep
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if calls != 4 {
		t.Fatalf("expected 4 calls, got %d", calls)
	}
}

func TestRetryNoRetryOnOtherErrors(t *testing.T) {
	calls := 0
	err := retryOnDBLockInternal(DefaultRetryConfig(), func() error {
		calls++
		return errors.New("unique constraint violated")
	}, func(d time.Duration) {})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retry), got %d", calls)
	}
}

func TestRetryExhaustsAllAttempts(t *testing.T) {
	calls := 0
	cfg := DefaultRetryConfig()
	err := retryOnDBLockInternal(cfg, func() error {
		calls++
		return errors.New("database is locked")
	}, func(d time.Duration) {})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	expected := 1 + cfg.MaxRetries // initial + retries
	if calls != expected {
		t.Fatalf("expected %d calls, got %d", expected, calls)
	}
}

func TestRetrySucceedsImmediately(t *testing.T) {
	calls := 0
	err := retryOnDBLockInternal(DefaultRetryConfig(), func() error {
		calls++
		return nil
	}, func(d time.Duration) {})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryJitterBounds(t *testing.T) {
	cfg := DefaultRetryConfig()
	var mu sync.Mutex
	var sleeps []time.Duration

	retryOnDBLockInternal(cfg, func() error {
		return errors.New("database is locked")
	}, func(d time.Duration) {
		mu.Lock()
		sleeps = append(sleeps, d)
		mu.Unlock()
	})

	if len(sleeps) != cfg.MaxRetries {
		t.Fatalf("expected %d sleeps, got %d", cfg.MaxRetries, len(sleeps))
	}

	for i, d := range sleeps {
		base := cfg.BaseDelay * (1 << i)
		maxJitter := time.Duration(float64(base) * cfg.JitterPct)
		if d < base || d > base+maxJitter {
			t.Errorf("sleep[%d] = %v, expected [%v, %v]", i, d, base, base+maxJitter)
		}
	}
}

func TestRetryExponentialBackoff(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 4, BaseDelay: 10 * time.Millisecond, JitterPct: 0}
	var sleeps []time.Duration

	retryOnDBLockInternal(cfg, func() error {
		return errors.New("database is locked")
	}, func(d time.Duration) {
		sleeps = append(sleeps, d)
	})

	expected := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond, 80 * time.Millisecond}
	for i, d := range sleeps {
		if d != expected[i] {
			t.Errorf("sleep[%d] = %v, expected %v", i, d, expected[i])
		}
	}
}
