package dispatch

import (
	"sync"
	"time"
)

// CircuitState describes breaker health.
type CircuitState struct {
	Open           bool      `json:"open"`
	FailureCount   int       `json:"failureCount"`
	LastFailure    time.Time `json:"lastFailure,omitempty"`
	OpenedAt       time.Time `json:"openedAt,omitempty"`
	SuccessClears  bool      `json:"successClears"`
}

// CircuitBreaker opens after consecutive failures and cools down afterward.
type CircuitBreaker struct {
	mu         sync.Mutex
	threshold  int
	cooldown   time.Duration
	failures   int
	openedAt   time.Time
	lastFail   time.Time
}

// NewCircuitBreaker creates a breaker with failure threshold and cooldown.
func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold < 1 {
		threshold = 1
	}
	return &CircuitBreaker{threshold: threshold, cooldown: cooldown}
}

// Open reports whether requests should be short-circuited.
func (b *CircuitBreaker) Open() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openedAt.IsZero() {
		return false
	}
	if time.Since(b.openedAt) >= b.cooldown {
		b.openedAt = time.Time{}
		b.failures = 0
		return false
	}
	return true
}

// RecordFailure increments failures and opens the breaker when threshold reached.
func (b *CircuitBreaker) RecordFailure(at time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	b.lastFail = at
	if b.failures >= b.threshold {
		b.openedAt = at
	}
}

// RecordSuccess clears failure count and closes an open breaker.
func (b *CircuitBreaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.openedAt = time.Time{}
}

// State returns a snapshot of breaker metrics.
func (b *CircuitBreaker) State(now time.Time) CircuitState {
	b.mu.Lock()
	defer b.mu.Unlock()
	open := !b.openedAt.IsZero()
	if open && now.Sub(b.openedAt) >= b.cooldown {
		open = false
	}
	return CircuitState{
		Open:          open,
		FailureCount:  b.failures,
		LastFailure:   b.lastFail,
		OpenedAt:      b.openedAt,
		SuccessClears: true,
	}
}
