package interlock

import (
	"fmt"
	"sync"
)

// Matrix stores mutual exclusion relationships between gates.
type Matrix struct {
	mu   sync.RWMutex
	ids  []string
	index map[string]int
	cells [][]bool
}

// NewMatrix builds an empty square matrix for the provided gate ids.
func NewMatrix(gateIDs []string) (*Matrix, error) {
	if len(gateIDs) == 0 {
		return nil, fmt.Errorf("at least one gate id is required")
	}
	index := make(map[string]int, len(gateIDs))
	for i, id := range gateIDs {
		if _, exists := index[id]; exists {
			return nil, fmt.Errorf("duplicate gate id %q", id)
		}
		index[id] = i
	}
	n := len(gateIDs)
	cells := make([][]bool, n)
	for i := range cells {
		cells[i] = make([]bool, n)
	}
	return &Matrix{ids: append([]string(nil), gateIDs...), index: index, cells: cells}, nil
}

// SetExclusive marks gates i and j as unable to be active simultaneously.
func (m *Matrix) SetExclusive(a, b string, exclusive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ia, okA := m.index[a]
	ib, okB := m.index[b]
	if !okA || !okB {
		return fmt.Errorf("unknown gate in matrix")
	}
	// The matrix is symmetric: a conflict between a and b must hold in both
	// directions, otherwise reverse queries (Exclusive(b, a)) would silently miss
	// a relationship registered via SetExclusive(a, b).
	m.cells[ia][ib] = exclusive
	m.cells[ib][ia] = exclusive
	return nil
}

// Exclusive reports whether two gates are mutually exclusive.
func (m *Matrix) Exclusive(a, b string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if a == b {
		return false, nil
	}
	ia, okA := m.index[a]
	ib, okB := m.index[b]
	if !okA || !okB {
		return false, fmt.Errorf("unknown gate in matrix")
	}
	return m.cells[ia][ib], nil
}

// GateIDs returns the ordered gate identifiers managed by the matrix.
func (m *Matrix) GateIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string(nil), m.ids...)
}

// Snapshot returns a deep copy of the matrix cells.
func (m *Matrix) Snapshot() [][]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([][]bool, len(m.cells))
	for i, row := range m.cells {
		out[i] = append([]bool(nil), row...)
	}
	return out
}

// DefaultPairwise builds a matrix where adjacent upstream/downstream gates conflict.
func DefaultPairwise(gateIDs []string) (*Matrix, error) {
	m, err := NewMatrix(gateIDs)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(gateIDs); i++ {
		for j := i + 1; j < len(gateIDs); j++ {
			if err := m.SetExclusive(gateIDs[i], gateIDs[j], true); err != nil {
				return nil, err
			}
		}
	}
	return m, nil
}
