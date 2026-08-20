package journal

import (
	"time"

	"github.com/lacsar712/tidegate/internal/model"
)

// MapDeny converts journal deny entries to API deny reasons.
func MapDeny(entries []Entry) []model.DenyReason {
	out := make([]model.DenyReason, 0, len(entries))
	for _, e := range entries {
		out = append(out, model.DenyReason{
			Code:      string(e.Kind),
			Message:   e.Message,
			ChamberID: e.ChamberID,
			GateID:    e.GateID,
			At:        e.At,
		})
	}
	return out
}

// RecordDeny appends a structured denial entry.
func RecordDeny(j *Journal, code, message, chamberID, gateID string, at time.Time) error {
	return j.Append(Entry{
		Kind:      KindDeny,
		ChamberID: chamberID,
		GateID:    gateID,
		Message:   code + ": " + message,
		At:        at,
	})
}
