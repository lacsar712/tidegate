package model

import (
	"fmt"
	"strings"
	"time"
)

// Chamber aggregates upstream/downstream levels and gate membership.
type Chamber struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	UpstreamLevelCM    int       `json:"upstreamLevelCm"`
	DownstreamLevelCM  int       `json:"downstreamLevelCm"`
	SmoothedUpstreamCM int       `json:"smoothedUpstreamCm"`
	SmoothedDownstream int       `json:"smoothedDownstreamCm"`
	HeadDiffCM         int       `json:"headDiffCm"`
	GateIDs            []string  `json:"gateIds"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// Clone returns a shallow copy with duplicated gate id slice.
func (c *Chamber) Clone() Chamber {
	if c == nil {
		return Chamber{}
	}
	out := *c
	if len(c.GateIDs) > 0 {
		out.GateIDs = append([]string(nil), c.GateIDs...)
	}
	return out
}

// ApplySmoothedLevels writes smoothed probe values and derived head diff.
func (c *Chamber) ApplySmoothedLevels(upstream, downstream, headDiff int, at time.Time) {
	c.SmoothedUpstreamCM = upstream
	c.SmoothedDownstream = downstream
	c.HeadDiffCM = headDiff
	c.UpdatedAt = at
}

// LevelReport is an ingest payload for a water-level probe.
type LevelReport struct {
	ChamberID   string    `json:"chamberId"`
	Probe       string    `json:"probe"`
	LevelCM     int       `json:"levelCm"`
	ReportedAt  time.Time `json:"reportedAt"`
}

const (
	ProbeUpstream   = "upstream"
	ProbeDownstream = "downstream"
)

// Validate ensures level report fields are present and sane.
func (r LevelReport) Validate() error {
	if strings.TrimSpace(r.ChamberID) == "" {
		return fmt.Errorf("chamberId is required")
	}
	switch strings.ToLower(strings.TrimSpace(r.Probe)) {
	case ProbeUpstream, ProbeDownstream:
	default:
		return fmt.Errorf("probe must be upstream or downstream")
	}
	if r.ReportedAt.IsZero() {
		return fmt.Errorf("reportedAt is required")
	}
	return nil
}

// ProbeSide normalizes the probe identifier.
func (r LevelReport) ProbeSide() string {
	return strings.ToLower(strings.TrimSpace(r.Probe))
}
