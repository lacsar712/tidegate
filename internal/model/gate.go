package model

import (
	"fmt"
	"strings"
	"time"
)

// GateState represents the operational state of a sluice gate.
type GateState string

const (
	GateClosed  GateState = "Closed"
	GateOpening GateState = "Opening"
	GateOpen    GateState = "Open"
	GateClosing GateState = "Closing"
	GateFault   GateState = "Fault"
)

// Active returns true when the gate is moving or fully open.
func (s GateState) Active() bool {
	return s == GateOpening || s == GateOpen
}

// String returns the canonical state label.
func (s GateState) String() string {
	return string(s)
}

// ParseGateState converts a string to GateState.
func ParseGateState(raw string) (GateState, error) {
	switch strings.TrimSpace(raw) {
	case string(GateClosed):
		return GateClosed, nil
	case string(GateOpening):
		return GateOpening, nil
	case string(GateOpen):
		return GateOpen, nil
	case string(GateClosing):
		return GateClosing, nil
	case string(GateFault):
		return GateFault, nil
	default:
		return "", fmt.Errorf("unknown gate state %q", raw)
	}
}

// Gate identifies a single sluice gate within a chamber.
type Gate struct {
	ID           string    `json:"id"`
	ChamberID    string    `json:"chamberId"`
	Name         string    `json:"name"`
	State        GateState `json:"state"`
	OpenPercent  int       `json:"openPercent"`
	InPosition   bool      `json:"inPosition"`
	FaultCode    int       `json:"faultCode"`
	LastReportAt time.Time `json:"lastReportAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Clone returns a deep copy of the gate snapshot fields.
func (g *Gate) Clone() Gate {
	if g == nil {
		return Gate{}
	}
	return *g
}

// Report describes an encoder telemetry payload.
type GateReport struct {
	GateID      string    `json:"gateId"`
	ChamberID   string    `json:"chamberId"`
	OpenPercent int       `json:"openPercent"`
	InPosition  bool      `json:"inPosition"`
	FaultCode   int       `json:"faultCode"`
	ReportedAt  time.Time `json:"reportedAt"`
}

// Validate checks mandatory gate report fields.
func (r GateReport) Validate() error {
	if strings.TrimSpace(r.GateID) == "" {
		return fmt.Errorf("gateId is required")
	}
	if strings.TrimSpace(r.ChamberID) == "" {
		return fmt.Errorf("chamberId is required")
	}
	if r.OpenPercent < 0 || r.OpenPercent > 100 {
		return fmt.Errorf("openPercent must be 0-100")
	}
	if r.ReportedAt.IsZero() {
		return fmt.Errorf("reportedAt is required")
	}
	return nil
}
