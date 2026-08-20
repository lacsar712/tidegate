package gatefsm

import (
	"fmt"

	"github.com/lacsar712/tidegate/internal/model"
)

var allowed = map[model.GateState]map[model.GateState]bool{
	model.GateClosed: {
		// Opening must be entered first to confirm stroke; a direct
		// Closed->Open jump is forbidden so the PLC never receives a
		// full-open command without the in-motion posture.
		model.GateOpening: true,
		model.GateFault:   true,
	},
	model.GateOpening: {
		model.GateOpen:  true,
		model.GateClosed: true,
		model.GateFault: true,
	},
	model.GateOpen: {
		model.GateClosing: true,
		model.GateFault:   true,
	},
	model.GateClosing: {
		model.GateClosed: true,
		model.GateOpen:   true,
		model.GateFault:  true,
	},
	model.GateFault: {
		model.GateClosed: true,
	},
}

// CanTransition reports whether from->to is legal.
func CanTransition(from, to model.GateState) bool {
	next, ok := allowed[from]
	if !ok {
		return false
	}
	return next[to]
}

// transition mutates gate state when legal.
func transition(g *model.Gate, from, to model.GateState) error {
	if from == to {
		return nil
	}
	if !CanTransition(from, to) {
		return fmt.Errorf("illegal gate transition %s -> %s", from, to)
	}
	g.State = to
	return nil
}

// AllowedTargets returns legal next states from the current state.
func AllowedTargets(from model.GateState) []model.GateState {
	next := allowed[from]
	out := make([]model.GateState, 0, len(next))
	for to := range next {
		out = append(out, to)
	}
	return out
}

// IsTerminalMotion reports whether the gate is actively moving.
func IsTerminalMotion(state model.GateState) bool {
	return state == model.GateOpening || state == model.GateClosing
}

// ConflictsWithInterlock returns true when state should block another gate opening.
func ConflictsWithInterlock(state model.GateState) bool {
	return state.Active()
}
