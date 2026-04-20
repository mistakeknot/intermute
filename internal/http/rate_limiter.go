package httpapi

import (
	"sync"
	"time"
)

// rateLimiter is a generic token-bucket-per-key rate limiter. Both the
// broadcast limiter (keyed on project+sender) and the live-transport
// limiter (keyed on sender+recipient) use this single implementation to
// avoid maintaining two parallel token-bucket structs with identical
// semantics.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rlBucket
	limit   int
	window  time.Duration
	// now is injectable for tests; defaults to time.Now.
	now func() time.Time
}

type rlBucket struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]*rlBucket),
		limit:   limit,
		window:  window,
	}
}

// allow returns true if key has budget remaining in the current window,
// false if the caller should be rate-limited.
func (l *rateLimiter) allow(key string) bool {
	t := time.Now
	if l.now != nil {
		t = l.now
	}
	now := t()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok || now.After(b.resetAt) {
		l.buckets[key] = &rlBucket{count: 1, resetAt: now.Add(l.window)}
		return true
	}
	b.count++
	return b.count <= l.limit
}
