package permit

import (
	"sync"
	"time"
)

// NonceLedger tracks spent nonces to prevent replay.
type NonceLedger struct {
	mu     sync.Mutex
	spent  map[string]time.Time
	ttl    time.Duration
}

// NewNonceLedger creates a replay ledger with optional expiry sweep cadence.
func NewNonceLedger(ttl time.Duration) *NonceLedger {
	return &NonceLedger{spent: make(map[string]time.Time), ttl: ttl}
}

// Spend records a nonce as used at the provided time.
func (l *NonceLedger) Spend(nonce string, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.spent[nonce]; exists {
		return ErrNonceReplay
	}
	l.spent[nonce] = at
	return nil
}

// Seen reports whether a nonce was already spent.
func (l *NonceLedger) Seen(nonce string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.spent[nonce]
	return ok
}

// Purge removes entries older than ttl relative to now.
func (l *NonceLedger) Purge(now time.Time) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	removed := 0
	for nonce, at := range l.spent {
		if now.Sub(at) > l.ttl {
			delete(l.spent, nonce)
			removed++
		}
	}
	return removed
}

// Count returns the number of tracked nonces.
func (l *NonceLedger) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.spent)
}
