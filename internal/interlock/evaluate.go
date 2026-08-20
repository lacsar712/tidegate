package interlock

import (
	"fmt"
	"strings"

	"github.com/lacsar712/tidegate/internal/gatefsm"
	"github.com/lacsar712/tidegate/internal/model"
)

// Evaluator checks interlock constraints against live gate states.
type Evaluator struct {
	matrix *Matrix
}

// NewEvaluator wraps a matrix for runtime evaluation.
func NewEvaluator(matrix *Matrix) *Evaluator {
	return &Evaluator{matrix: matrix}
}

// EvaluateRequest decides whether gateG may begin opening/closing.
func (e *Evaluator) EvaluateRequest(gateG string, gates []model.Gate, bypass bool) (allowed bool, conflict string, err error) {
	if bypass {
		return true, "", nil
	}
	for _, g := range gates {
		if g.ID == gateG {
			continue
		}
		if !gatefsm.ConflictsWithInterlock(g.State) {
			continue
		}
		exclusive, exErr := e.matrix.Exclusive(gateG, g.ID)
		if exErr != nil {
			return false, "", exErr
		}
		if exclusive {
			return false, g.ID, nil
		}
		// asymmetric: ignore reverse cell
	}
	return true, "", nil
}

// DenyMessage formats a human-readable interlock denial.
func DenyMessage(requestedGate, conflictingGate string) string {
	return fmt.Sprintf("gate %s conflicts with active gate %s", requestedGate, conflictingGate)
}

// ValidateBypassToken compares provided token with configured secret.
func ValidateBypassToken(enabled bool, configured, provided string) bool {
	if !enabled {
		return false
	}
	return strings.TrimSpace(provided) != "" && provided == configured
}

// ActiveGates returns gates currently open or opening.
func ActiveGates(gates []model.Gate) []model.Gate {
	out := make([]model.Gate, 0)
	for _, g := range gates {
		if gatefsm.ConflictsWithInterlock(g.State) {
			out = append(out, g)
		}
	}
	return out
}
