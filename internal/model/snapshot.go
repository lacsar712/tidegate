package model

import "time"

// GateView is the public projection of a gate for APIs and UI.
type GateView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	State       GateState `json:"state"`
	OpenPercent int       `json:"openPercent"`
	InPosition  bool      `json:"inPosition"`
	FaultCode   int       `json:"faultCode"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ChamberView is the public chamber snapshot.
type ChamberView struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	UpstreamLevelCM    int        `json:"upstreamLevelCm"`
	DownstreamLevelCM  int        `json:"downstreamLevelCm"`
	SmoothedUpstreamCM int        `json:"smoothedUpstreamCm"`
	SmoothedDownstream int        `json:"smoothedDownstreamCm"`
	HeadDiffCM         int        `json:"headDiffCm"`
	Gates              []GateView `json:"gates"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// StatusSnapshot aggregates the full operator-visible state.
type StatusSnapshot struct {
	GeneratedAt time.Time     `json:"generatedAt"`
	Chambers    []ChamberView `json:"chambers"`
	RecentDeny  []DenyReason  `json:"recentDeny"`
}

// PermitResult is returned after permit evaluation and optional dispatch.
type PermitResult struct {
	Allowed bool          `json:"allowed"`
	Ticket  *PermitTicket `json:"ticket,omitempty"`
	Deny    *DenyReason   `json:"deny,omitempty"`
}

// DispatchOutcome records PLC command results.
type DispatchOutcome struct {
	GateID     string    `json:"gateId"`
	Action     string    `json:"action"`
	Success    bool      `json:"success"`
	StatusCode int       `json:"statusCode"`
	Message    string    `json:"message"`
	Attempted  time.Time `json:"attempted"`
}
