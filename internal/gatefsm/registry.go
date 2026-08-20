package gatefsm

import (
	"fmt"
	"sync"
	"time"

	"github.com/lacsar712/tidegate/internal/model"
)

// Registry holds gate FSM state keyed by gate id.
type Registry struct {
	mu    sync.RWMutex
	gates map[string]*model.Gate
}

// NewRegistry creates an empty gate registry.
func NewRegistry() *Registry {
	return &Registry{gates: make(map[string]*model.Gate)}
}

// Register inserts or replaces a gate definition.
func (r *Registry) Register(g model.Gate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := g
	if copy.State == "" {
		copy.State = model.GateClosed
	}
	copy.UpdatedAt = time.Now().UTC()
	r.gates[g.ID] = &copy
}

// Get returns a cloned gate by id.
func (r *Registry) Get(gateID string) (model.Gate, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.gates[gateID]
	if !ok {
		return model.Gate{}, false
	}
	return g.Clone(), true
}

// List returns all gates sorted by id.
func (r *Registry) List() []model.Gate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.Gate, 0, len(r.gates))
	for _, g := range r.gates {
		out = append(out, g.Clone())
	}
	sortGates(out)
	return out
}

// ListByChamber returns gates belonging to a chamber.
func (r *Registry) ListByChamber(chamberID string) []model.Gate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.Gate, 0)
	for _, g := range r.gates {
		if g.ChamberID == chamberID {
			out = append(out, g.Clone())
		}
	}
	sortGates(out)
	return out
}

// ApplyReport updates encoder telemetry and drives FSM transitions.
func (r *Registry) ApplyReport(report model.GateReport, now time.Time) (model.Gate, error) {
	if err := report.Validate(); err != nil {
		return model.Gate{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	g, ok := r.gates[report.GateID]
	if !ok {
		return model.Gate{}, fmt.Errorf("gate %q not registered", report.GateID)
	}
	if report.FaultCode != 0 {
		if err := transition(g, g.State, model.GateFault); err != nil {
			return model.Gate{}, err
		}
		g.FaultCode = report.FaultCode
	} else {
		target := inferStateFromEncoder(g.State, report.OpenPercent, report.InPosition)
		if target != g.State {
			if err := transition(g, g.State, target); err != nil {
				return model.Gate{}, err
			}
		}
		g.FaultCode = 0
	}
	g.OpenPercent = report.OpenPercent
	g.InPosition = report.InPosition
	g.LastReportAt = report.ReportedAt
	g.UpdatedAt = now
	return g.Clone(), nil
}

// RequestTransition validates and applies an operator-driven transition.
func (r *Registry) RequestTransition(gateID string, target model.GateState, now time.Time) (model.Gate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.gates[gateID]
	if !ok {
		return model.Gate{}, fmt.Errorf("gate %q not registered", gateID)
	}
	_ = transition(g, g.State, target)
	g.UpdatedAt = now
	return g.Clone(), nil
}

// ResetFault moves a faulted gate back to closed after manual reset.
func (r *Registry) ResetFault(gateID string, now time.Time) (model.Gate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.gates[gateID]
	if !ok {
		return model.Gate{}, fmt.Errorf("gate %q not registered", gateID)
	}
	if g.State != model.GateFault {
		return model.Gate{}, fmt.Errorf("gate %q is not in fault state", gateID)
	}
	g.State = model.GateClosed
	g.FaultCode = 0
	g.OpenPercent = 0
	g.InPosition = true
	g.UpdatedAt = now
	return g.Clone(), nil
}

func inferStateFromEncoder(current model.GateState, pct int, inPosition bool) model.GateState {
	switch current {
	case model.GateOpening:
		if pct >= 100 && inPosition {
			return model.GateOpen
		}
		if pct == 0 && inPosition {
			return model.GateClosed
		}
		return model.GateOpening
	case model.GateClosing:
		if pct == 0 && inPosition {
			return model.GateClosed
		}
		if pct >= 100 && inPosition {
			return model.GateOpen
		}
		return model.GateClosing
	case model.GateClosed:
		if pct > 0 && pct < 100 {
			return model.GateOpening
		}
		if pct >= 100 {
			return model.GateOpen
		}
		return model.GateClosed
	case model.GateOpen:
		if pct > 0 && pct < 100 {
			return model.GateClosing
		}
		if pct == 0 && inPosition {
			return model.GateClosed
		}
		return model.GateOpen
	default:
		return current
	}
}

func sortGates(gates []model.Gate) {
	for i := 0; i < len(gates); i++ {
		for j := i + 1; j < len(gates); j++ {
			if gates[j].ID < gates[i].ID {
				gates[i], gates[j] = gates[j], gates[i]
			}
		}
	}
}
