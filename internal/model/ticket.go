package model

import (
	"fmt"
	"strings"
	"time"
)

// PermitAction describes whether a ticket authorizes open or close motion.
type PermitAction string

const (
	ActionOpen  PermitAction = "open"
	ActionClose PermitAction = "close"
)

// ParsePermitAction converts user input to a permit action.
func ParsePermitAction(raw string) (PermitAction, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ActionOpen):
		return ActionOpen, nil
	case string(ActionClose):
		return ActionClose, nil
	default:
		return "", fmt.Errorf("unknown permit action %q", raw)
	}
}

// PermitTicket authorizes one gate motion until expiry.
type PermitTicket struct {
	ID        string       `json:"id"`
	ChamberID string       `json:"chamberId"`
	GateID    string       `json:"gateId"`
	Action    PermitAction `json:"action"`
	Nonce     string       `json:"nonce"`
	IssuedAt  time.Time    `json:"issuedAt"`
	ExpireAt  time.Time    `json:"expireAt"`
	Used      bool         `json:"used"`
}

// Valid reports whether the ticket is still within its TTL and unused.
func (t PermitTicket) Valid(now time.Time) bool {
	return !t.Used && now.Before(t.ExpireAt)
}

// Expired reports whether the ticket TTL has elapsed.
func (t PermitTicket) Expired(now time.Time) bool {
	return !now.Before(t.ExpireAt)
}

// OpenRequest is an operator/API request to begin gate motion.
type OpenRequest struct {
	ChamberID   string       `json:"chamberId"`
	GateID      string       `json:"gateId"`
	Action      PermitAction `json:"action"`
	BypassToken string       `json:"bypassToken,omitempty"`
}

// Validate checks mandatory request fields.
func (r OpenRequest) Validate() error {
	if strings.TrimSpace(r.ChamberID) == "" {
		return fmt.Errorf("chamberId is required")
	}
	if strings.TrimSpace(r.GateID) == "" {
		return fmt.Errorf("gateId is required")
	}
	if r.Action != ActionOpen && r.Action != ActionClose {
		return fmt.Errorf("action must be open or close")
	}
	return nil
}

// DenyReason captures why a permit was rejected.
type DenyReason struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	ChamberID string    `json:"chamberId,omitempty"`
	GateID    string    `json:"gateId,omitempty"`
	At        time.Time `json:"at"`
}
