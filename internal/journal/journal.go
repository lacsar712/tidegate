package journal

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Kind categorizes journal records.
type Kind string

const (
	KindLevel   Kind = "LEVEL"
	KindGate    Kind = "GATE"
	KindPermit  Kind = "PERMIT"
	KindDeny    Kind = "DENY"
	KindBypass  Kind = "BYPASS"
	KindReject  Kind = "REJECT"
	KindDispatch Kind = "DISPATCH"
)

// Entry is one append-only journal line.
type Entry struct {
	Kind      Kind      `json:"kind"`
	ChamberID string    `json:"chamberId,omitempty"`
	GateID    string    `json:"gateId,omitempty"`
	Message   string    `json:"message"`
	At        time.Time `json:"at"`
}

// Journal persists operational records to disk and memory.
type Journal struct {
	mu       sync.RWMutex
	path     string
	maxRecent int
	recent   []Entry
	file     *os.File
}

// NewJournal opens or creates the journal file.
func NewJournal(path string, maxRecent int) (*Journal, error) {
	if maxRecent < 1 {
		maxRecent = 50
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open journal: %w", err)
	}
	return &Journal{path: path, maxRecent: maxRecent, file: f}, nil
}

// Append writes an entry to disk and retains recent copies in memory.
func (j *Journal) Append(e Entry) error {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if _, err := j.file.Write(append(line, '\n')); err != nil {
		return err
	}
	j.recent = append(j.recent, e)
	if len(j.recent) > j.maxRecent {
		j.recent = j.recent[len(j.recent)-j.maxRecent:]
	}
	return nil
}

// RecentDenials returns latest deny/reject entries for the operator UI.
func (j *Journal) RecentDenials(limit int) []Entry {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if limit <= 0 {
		limit = 10
	}
	out := make([]Entry, 0, limit)
	for i := len(j.recent) - 1; i >= 0 && len(out) < limit; i-- {
		e := j.recent[i]
		if e.Kind == KindDeny || e.Kind == KindReject {
			out = append(out, e)
		}
	}
	return out
}

// Recent returns the last n entries of any kind.
func (j *Journal) Recent(limit int) []Entry {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if limit <= 0 || limit > len(j.recent) {
		limit = len(j.recent)
	}
	start := len(j.recent) - limit
	if start < 0 {
		start = 0
	}
	cp := make([]Entry, limit)
	copy(cp, j.recent[start:])
	return cp
}

// Close flushes and closes the underlying file.
func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil {
		return nil
	}
	err := j.file.Close()
	j.file = nil
	return err
}

// Path returns the on-disk journal location.
func (j *Journal) Path() string {
	return j.path
}
