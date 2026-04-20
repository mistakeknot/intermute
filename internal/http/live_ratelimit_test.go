package httpapi

import (
	"fmt"
	"testing"
	"time"
)

func newLiveTestLimiter() *rateLimiter {
	return newRateLimiter(liveRateLimit, liveRateWindow)
}

// liveAllow mirrors Service.liveAllow so tests exercise the same key
// composition as production.
func liveTestAllow(l *rateLimiter, sender, recipient string) bool {
	return l.allow(sender + ":" + recipient)
}

func TestLiveRateLimiterAllowsTenPerPairPerMinute(t *testing.T) {
	limiter := newLiveTestLimiter()
	for i := 0; i < liveRateLimit; i++ {
		if !liveTestAllow(limiter, "alice", "bob") {
			t.Fatalf("request %d unexpectedly denied", i+1)
		}
	}
	if liveTestAllow(limiter, "alice", "bob") {
		t.Fatal("11th request unexpectedly allowed")
	}
}

func TestLiveRateLimiterIsScopedPerPair(t *testing.T) {
	limiter := newLiveTestLimiter()
	for i := 0; i < liveRateLimit; i++ {
		if !liveTestAllow(limiter, "alice", "bob") {
			t.Fatalf("alice->bob request %d unexpectedly denied", i+1)
		}
	}
	if liveTestAllow(limiter, "alice", "bob") {
		t.Fatal("alice->bob overflow unexpectedly allowed")
	}
	if !liveTestAllow(limiter, "alice", "carol") {
		t.Fatal("alice->carol should use a separate bucket")
	}
	if !liveTestAllow(limiter, "dave", "bob") {
		t.Fatal("dave->bob should use a separate bucket")
	}
}

func TestLiveRateLimiterResetsAfterWindow(t *testing.T) {
	base := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	now := base
	limiter := newLiveTestLimiter()
	limiter.now = func() time.Time { return now }

	for i := 0; i < liveRateLimit; i++ {
		if !liveTestAllow(limiter, "alice", "bob") {
			t.Fatalf("request %d unexpectedly denied", i+1)
		}
	}
	if liveTestAllow(limiter, "alice", "bob") {
		t.Fatal("overflow unexpectedly allowed before reset")
	}

	now = base.Add(liveRateWindow + time.Second)
	if !liveTestAllow(limiter, "alice", "bob") {
		t.Fatal("request after reset window should be allowed")
	}
}

func TestLiveRateLimiterParallelSafety(t *testing.T) {
	limiter := newLiveTestLimiter()
	done := make(chan error, liveRateLimit)

	for i := 0; i < liveRateLimit; i++ {
		go func(i int) {
			if !liveTestAllow(limiter, "alice", fmt.Sprintf("bob-%d", i)) {
				done <- fmt.Errorf("request %d unexpectedly denied", i)
				return
			}
			done <- nil
		}(i)
	}

	for i := 0; i < liveRateLimit; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}
