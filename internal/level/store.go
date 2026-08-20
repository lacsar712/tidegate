package level

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/lacsar712/tidegate/internal/model"
)

// Sample holds one probe reading with ingest metadata.
type Sample struct {
	LevelCM    int
	ReportedAt time.Time
	ReceivedAt time.Time
}

// ProbeBuffer stores recent raw samples for median smoothing.
type ProbeBuffer struct {
	maxN     int
	samples  []Sample
	lastSeen time.Time
}

// NewProbeBuffer creates a buffer retaining the last maxN samples.
func NewProbeBuffer(maxN int) *ProbeBuffer {
	if maxN < 1 {
		maxN = 1
	}
	return &ProbeBuffer{maxN: maxN}
}

// Add appends a sample and trims to the configured window.
func (b *ProbeBuffer) Add(s Sample) {
	b.samples = append(b.samples, s)
	if len(b.samples) > b.maxN {
		b.samples = b.samples[len(b.samples)-b.maxN:]
	}
	b.lastSeen = s.ReceivedAt
}

// Median returns the median level across retained samples.
func (b *ProbeBuffer) Median() (int, bool) {
	if len(b.samples) == 0 {
		return 0, false
	}
	vals := make([]int, len(b.samples))
	for i, s := range b.samples {
		vals[i] = s.LevelCM
	}
	sort.Ints(vals)
	mid := len(vals) / 2
	if len(vals)%2 == 1 {
		return vals[mid], true
	}
	return (vals[mid-1] + vals[mid]) / 2, true
}

// Count returns the number of retained samples.
func (b *ProbeBuffer) Count() int {
	return len(b.samples)
}

// Last returns the most recently added sample level without smoothing.
func (b *ProbeBuffer) Last() (int, bool) {
	if len(b.samples) == 0 {
		return 0, false
	}
	return b.samples[len(b.samples)-1].LevelCM, true
}

// ChamberLevels tracks upstream and downstream probe buffers.
type ChamberLevels struct {
	Upstream   *ProbeBuffer
	Downstream *ProbeBuffer
}

// Store maintains per-chamber level smoothing state.
type Store struct {
	mu       sync.RWMutex
	smoothN  int
	skewSec  int
	chambers map[string]*ChamberLevels
}

// NewStore constructs a level store with smoothing and skew parameters.
func NewStore(smoothN, skewSec int) *Store {
	return &Store{
		smoothN:  smoothN,
		skewSec:  skewSec,
		chambers: make(map[string]*ChamberLevels),
	}
}

// EnsureChamber registers buffers for a chamber if missing.
func (s *Store) EnsureChamber(chamberID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLocked(chamberID)
}

func (s *Store) ensureLocked(chamberID string) *ChamberLevels {
	if cl, ok := s.chambers[chamberID]; ok {
		return cl
	}
	cl := &ChamberLevels{
		Upstream:   NewProbeBuffer(s.smoothN),
		Downstream: NewProbeBuffer(s.smoothN),
	}
	s.chambers[chamberID] = cl
	return cl
}

// Ingest validates skew, records the sample, and returns smoothed values.
func (s *Store) Ingest(report model.LevelReport, now time.Time) (upstream, downstream int, headDiff int, err error) {
	if err = report.Validate(); err != nil {
		return 0, 0, 0, err
	}
	skew := now.Sub(report.ReportedAt)
	if skew < 0 {
		skew = -skew
	}
	if int(skew.Seconds()) > s.skewSec {
		return 0, 0, 0, fmt.Errorf("sample timestamp skew exceeds %ds", s.skewSec)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cl := s.ensureLocked(report.ChamberID)
	sample := Sample{
		LevelCM:    report.LevelCM,
		ReportedAt: report.ReportedAt,
		ReceivedAt: now,
	}
	switch report.ProbeSide() {
	case model.ProbeUpstream:
		cl.Upstream.Add(sample)
	case model.ProbeDownstream:
		cl.Downstream.Add(sample)
	}

	up, okUp := cl.Upstream.Last()
	down, okDown := cl.Downstream.Last()
	if !okUp || !okDown {
		return 0, 0, 0, fmt.Errorf("both probes require samples before smoothing")
	}
	return up, down, HeadDiff(up, down), nil
}

// Snapshot reads smoothed medians without ingesting new data.
func (s *Store) Snapshot(chamberID string) (upstream, downstream, headDiff int, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cl, exists := s.chambers[chamberID]
	if !exists {
		return 0, 0, 0, false
	}
	up, okUp := cl.Upstream.Median()
	down, okDown := cl.Downstream.Median()
	if !okUp || !okDown {
		return 0, 0, 0, false
	}
	return up, down, HeadDiff(up, down), true
}
