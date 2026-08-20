package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lacsar712/tidegate/internal/gatefsm"
	"github.com/lacsar712/tidegate/internal/journal"
	"github.com/lacsar712/tidegate/internal/level"
	"github.com/lacsar712/tidegate/internal/model"
)

// RateLimiter provides a simple per-minute request cap.
type RateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Time
	count    int
}

// NewRateLimiter creates a limiter allowing limit requests per minute.
func NewRateLimiter(limit int) *RateLimiter {
	return &RateLimiter{limit: limit, window: time.Now().UTC()}
}

// Allow reports whether another request may proceed.
func (r *RateLimiter) Allow(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if now.Sub(r.window) >= time.Minute {
		r.window = now
		r.count = 0
	}
	if r.count >= r.limit {
		return false
	}
	r.count++
	return true
}

// Handler wires authenticated sensor ingest endpoints.
type Handler struct {
	verifier *Verifier
	levels   *level.Store
	gates    *gatefsm.Registry
	journal  *journal.Journal
	limiter  *RateLimiter
	now      func() time.Time
}

// NewHandler constructs sensor HTTP handlers.
func NewHandler(verifier *Verifier, levels *level.Store, gates *gatefsm.Registry, j *journal.Journal, limit int) *Handler {
	return &Handler{
		verifier: verifier,
		levels:   levels,
		gates:    gates,
		journal:  j,
		limiter:  NewRateLimiter(limit),
		now:      time.Now,
	}
}

// Mount registers ingest routes on mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/sensors/level", h.handleLevel)
	mux.HandleFunc("POST /v1/sensors/gate", h.handleGate)
}

func (h *Handler) handleLevel(w http.ResponseWriter, r *http.Request) {
	if !h.limiter.Allow(h.now().UTC()) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}
	body, _, err := h.readSigned(r)
	if err != nil {
		writeJSON(w, statusForIngestErr(err), map[string]string{"error": err.Error()})
		return
	}
	var report model.LevelReport
	if err := json.Unmarshal(body, &report); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	now := h.now().UTC()
	up, down, head, err := h.levels.Ingest(report, now)
	if err != nil {
		_ = h.journal.Append(journal.Entry{
			Kind:      journal.KindReject,
			ChamberID: report.ChamberID,
			Message:   err.Error(),
			At:        now,
		})
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	_ = h.journal.Append(journal.Entry{
		Kind:      journal.KindLevel,
		ChamberID: report.ChamberID,
		Message:   fmt.Sprintf("probe=%s smoothed up=%d down=%d head=%d", report.ProbeSide(), up, down, head),
		At:        now,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"upstreamCm":   up,
		"downstreamCm": down,
		"headDiffCm":   head,
	})
}

func (h *Handler) handleGate(w http.ResponseWriter, r *http.Request) {
	if !h.limiter.Allow(h.now().UTC()) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}
	body, _, err := h.readSigned(r)
	if err != nil {
		writeJSON(w, statusForIngestErr(err), map[string]string{"error": err.Error()})
		return
	}
	var report model.GateReport
	if err := json.Unmarshal(body, &report); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	now := h.now().UTC()
	gate, err := h.gates.ApplyReport(report, now)
	if err != nil {
		_ = h.journal.Append(journal.Entry{
			Kind:    journal.KindReject,
			GateID:  report.GateID,
			Message: err.Error(),
			At:      now,
		})
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	_ = h.journal.Append(journal.Entry{
		Kind:    journal.KindGate,
		GateID:  gate.ID,
		Message: fmt.Sprintf("state=%s open=%d%%", gate.State, gate.OpenPercent),
		At:      now,
	})
	writeJSON(w, http.StatusOK, gate)
}

func (h *Handler) readSigned(r *http.Request) ([]byte, map[string]string, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, nil, err
	}
	headers := map[string]string{
		headerKey:  r.Header.Get(headerKey),
		headerTime: r.Header.Get(headerTime),
	}
	if err := h.verifier.Verify(headers, body, h.now().UTC()); err != nil {
		return nil, headers, err
	}
	return body, headers, nil
}

func statusForIngestErr(err error) int {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "hmac"), strings.Contains(msg, "header"), strings.Contains(msg, "skew"):
		return http.StatusUnauthorized
	default:
		return http.StatusBadRequest
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
