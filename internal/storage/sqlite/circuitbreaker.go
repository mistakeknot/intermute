package sqlite

import (
	"errors"
	"sync"
	"time"
)

// BreakerState represents the state of the circuit breaker.
type BreakerState int

const (
	StateClosed   BreakerState = 0
	StateOpen     BreakerState = 1
	StateHalfOpen BreakerState = 2
)

// String returns the string representation of the breaker state.
func (s BreakerState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen is returned when the circuit breaker is open and rejecting requests.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreaker implements a 3-state circuit breaker pattern for SQLite resilience.
// States: CLOSED (normal) -> OPEN (failing) -> HALF_OPEN (probing) -> CLOSED.
type CircuitBreaker struct {
	mu           sync.Mutex
	state        BreakerState
	failures     int
	threshold    int
	resetTimeout time.Duration
	lastFailure  time.Time
	nowFunc      func() time.Time // for testing
}

// NewCircuitBreaker creates a circuit breaker with the given threshold and reset timeout.
func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:    threshold,
		resetTimeout: resetTimeout,
		nowFunc:      time.Now,
	}
}

// Execute runs fn through the circuit breaker. Returns ErrCircuitOpen if the
// breaker is open and the reset timeout hasn't elapsed.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	switch cb.state {
	case StateClosed:
		cb.mu.Unlock()
		err := fn()
		cb.mu.Lock()
		if err != nil {
			cb.failures++
			if cb.failures >= cb.threshold {
				cb.state = StateOpen
				cb.lastFailure = cb.nowFunc()
			}
		} else {
			cb.failures = 0
		}
		cb.mu.Unlock()
		return err

	case StateOpen:
		if cb.nowFunc().Sub(cb.lastFailure) >= cb.resetTimeout {
			// Transition to half-open: allow one probe request
			cb.state = StateHalfOpen
			cb.mu.Unlock()
			err := fn()
			cb.mu.Lock()
			if err != nil {
				cb.state = StateOpen
				cb.lastFailure = cb.nowFunc()
			} else {
				cb.state = StateClosed
				cb.failures = 0
			}
			cb.mu.Unlock()
			return err
		}
		cb.mu.Unlock()
		return ErrCircuitOpen

	case StateHalfOpen:
		// Only one probe allowed per reset cycle (the OPEN->HALF_OPEN transition)
		cb.mu.Unlock()
		return ErrCircuitOpen

	default:
		cb.mu.Unlock()
		return ErrCircuitOpen
	}
}

// State returns the current breaker state.
func (cb *CircuitBreaker) State() BreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
