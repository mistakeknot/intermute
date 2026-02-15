package sqlite

import (
	"math/rand/v2"
	"strings"
	"time"
)

// RetryConfig controls exponential backoff retry behavior.
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	JitterPct  float64 // e.g. 0.25 for 25% jitter
}

// DefaultRetryConfig returns the default retry configuration:
// 7 retries, 50ms base, 25% jitter.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 7,
		BaseDelay:  50 * time.Millisecond,
		JitterPct:  0.25,
	}
}

// RetryOnDBLock retries fn on "database is locked" errors using default config.
func RetryOnDBLock(fn func() error) error {
	return retryOnDBLockInternal(DefaultRetryConfig(), fn, time.Sleep)
}

// RetryOnDBLockWithConfig retries fn on "database is locked" errors using the given config.
func RetryOnDBLockWithConfig(cfg RetryConfig, fn func() error) error {
	return retryOnDBLockInternal(cfg, fn, time.Sleep)
}

func retryOnDBLockInternal(cfg RetryConfig, fn func() error, sleepFn func(time.Duration)) error {
	err := fn()
	if err == nil {
		return nil
	}
	if !isDBLocked(err) {
		return err
	}

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		delay := cfg.BaseDelay * (1 << (attempt - 1))
		jitter := time.Duration(float64(delay) * rand.Float64() * cfg.JitterPct)
		sleepFn(delay + jitter)

		err = fn()
		if err == nil {
			return nil
		}
		if !isDBLocked(err) {
			return err
		}
	}
	return err
}

func isDBLocked(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "database is locked")
}
