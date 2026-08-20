package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/lacsar712/tidegate/internal/config"
	"github.com/lacsar712/tidegate/internal/dispatch"
	"github.com/lacsar712/tidegate/internal/gatefsm"
	"github.com/lacsar712/tidegate/internal/ingest"
	"github.com/lacsar712/tidegate/internal/interlock"
	"github.com/lacsar712/tidegate/internal/journal"
	"github.com/lacsar712/tidegate/internal/level"
	"github.com/lacsar712/tidegate/internal/model"
	"github.com/lacsar712/tidegate/internal/permit"
	"github.com/lacsar712/tidegate/internal/web"
)

// App wires domain modules into a runnable HTTP service.
type App struct {
	cfg       config.Config
	mux       *http.ServeMux
	server    *http.Server
	levels    *level.Store
	gates     *gatefsm.Registry
	chambers  map[string]model.Chamber
	chamberMu sync.RWMutex
	matrix    *interlock.Matrix
	evaluator *interlock.Evaluator
	permits   *permit.Service
	ingest    *ingest.Handler
	dispatch  *dispatch.PLCClient
	journal   *journal.Journal
	now       func() time.Time
}

// New constructs the tidegate application graph.
func New(cfg config.Config) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	j, err := journal.NewJournal(cfg.JournalPath, 100)
	if err != nil {
		return nil, err
	}
	verifier, err := ingest.NewVerifier(cfg.HMACSecret, cfg.SkewSec)
	if err != nil {
		_ = j.Close()
		return nil, err
	}

	levels := level.NewStore(cfg.SmoothN, cfg.SkewSec)
	gates := gatefsm.NewRegistry()

	gateDefs := []model.Gate{
		{ID: "gate-upstream", ChamberID: cfg.DefaultChamberID, Name: "Upstream Gate", State: model.GateClosed},
		{ID: "gate-downstream", ChamberID: cfg.DefaultChamberID, Name: "Downstream Gate", State: model.GateClosed},
	}
	gateIDs := make([]string, 0, len(gateDefs))
	for _, g := range gateDefs {
		gates.Register(g)
		gateIDs = append(gateIDs, g.ID)
	}

	matrix, err := interlock.DefaultPairwise(gateIDs)
	if err != nil {
		_ = j.Close()
		return nil, err
	}
	evaluator := interlock.NewEvaluator(matrix)
	permits := permit.NewService(levels, evaluator, cfg.HeadDiffLimitCM, cfg.TicketTTL, cfg.BypassEnabled, cfg.BypassToken)
	plc := dispatch.NewPLCClient(cfg.PLCEndpoint, cfg.PLCTimeout, cfg.PLCRetries, cfg.CircuitThreshold, cfg.CircuitCooldown)
	ingestHandler := ingest.NewHandler(verifier, levels, gates, j, cfg.RateLimitPerMinute)

	chamber := model.Chamber{
		ID:      cfg.DefaultChamberID,
		Name:    "Main Chamber",
		GateIDs: gateIDs,
	}
	levels.EnsureChamber(chamber.ID)

	app := &App{
		cfg:       cfg,
		mux:       http.NewServeMux(),
		levels:    levels,
		gates:     gates,
		chambers:  map[string]model.Chamber{chamber.ID: chamber},
		matrix:    matrix,
		evaluator: evaluator,
		permits:   permits,
		ingest:    ingestHandler,
		dispatch:  plc,
		journal:   j,
		now:       time.Now,
	}
	app.mountRoutes()
	app.server = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return app, nil
}

func (a *App) mountRoutes() {
	a.ingest.Mount(a.mux)
	a.mux.HandleFunc("GET /v1/status", a.handleStatus)
	a.mux.HandleFunc("POST /v1/ops/request-open", a.handleRequestOpen)
	a.mux.HandleFunc("POST /v1/ops/reset-fault", a.handleResetFault)
	web.Mount(a.mux)
}

// Run starts the HTTP server until context cancellation.
func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.server.Shutdown(shutdownCtx)
		return a.Close()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// Close releases journal resources.
func (a *App) Close() error {
	return a.journal.Close()
}

// Handler exposes the root mux for tests.
func (a *App) Handler() http.Handler {
	return a.mux
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	snap := a.buildSnapshot(a.now().UTC())
	writeJSON(w, http.StatusOK, snap)
}

func (a *App) handleRequestOpen(w http.ResponseWriter, r *http.Request) {
	var req model.OpenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	now := a.now().UTC()
	gates := a.gates.ListByChamber(req.ChamberID)
	result := a.permits.EvaluateOpenRequest(req, gates, now)
	if !result.Allowed {
		deny := result.Deny
		_ = a.journal.Append(journal.Entry{
			Kind:      journal.KindDeny,
			ChamberID: req.ChamberID,
			GateID:    req.GateID,
			Message:   fmt.Sprintf("%s: %s", deny.Code, deny.Message),
			At:        now,
		})
		writeJSON(w, http.StatusForbidden, result)
		return
	}
	if interlock.ValidateBypassToken(a.cfg.BypassEnabled, a.cfg.BypassToken, req.BypassToken) {
		_ = a.journal.Append(journal.Entry{
			Kind:      journal.KindBypass,
			ChamberID: req.ChamberID,
			GateID:    req.GateID,
			Message:   "interlock bypass accepted",
			At:        now,
		})
	}

	ticket := *result.Ticket
	target := model.GateOpen
	if req.Action == model.ActionClose {
		target = model.GateClosing
	}
	if _, err := a.gates.RequestTransition(req.GateID, target, now); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), a.cfg.PLCTimeout*time.Duration(a.cfg.PLCRetries+2))
	defer cancel()
	outcome, err := a.dispatch.Send(ctx, ticket)
	if err != nil {
		_ = a.journal.Append(journal.Entry{
			Kind:      journal.KindDispatch,
			ChamberID: req.ChamberID,
			GateID:    req.GateID,
			Message:   fmt.Sprintf("dispatch failed: %v", err),
			At:        now,
		})
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	consumed, err := a.permits.Consume(ticket.ID, now)
	if err != nil {
		writeJSON(w, permitHTTPStatus(err), map[string]string{"error": err.Error()})
		return
	}
	_ = a.journal.Append(journal.Entry{
		Kind:      journal.KindPermit,
		ChamberID: req.ChamberID,
		GateID:    req.GateID,
		Message:   fmt.Sprintf("issued action=%s nonce=%s", consumed.Action, consumed.Nonce),
		At:        now,
	})
	_ = a.journal.Append(journal.Entry{
		Kind:      journal.KindDispatch,
		ChamberID: req.ChamberID,
		GateID:    req.GateID,
		Message:   fmt.Sprintf("plc status=%d success=%v", outcome.StatusCode, outcome.Success),
		At:        now,
	})
	a.updateChamberLevels(req.ChamberID, now)
	writeJSON(w, http.StatusOK, map[string]any{"ticket": consumed, "dispatch": outcome})
}

func (a *App) handleResetFault(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		GateID string `json:"gateId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	now := a.now().UTC()
	gate, err := a.gates.ResetFault(payload.GateID, now)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, gate)
}

func (a *App) buildSnapshot(now time.Time) model.StatusSnapshot {
	a.chamberMu.RLock()
	defer a.chamberMu.RUnlock()
	chViews := make([]model.ChamberView, 0, len(a.chambers))
	for _, ch := range a.chambers {
		view := model.ChamberView{
			ID:                ch.ID,
			Name:              ch.Name,
			UpstreamLevelCM:   ch.UpstreamLevelCM,
			DownstreamLevelCM: ch.DownstreamLevelCM,
			UpdatedAt:         ch.UpdatedAt,
		}
		if up, down, head, ok := a.levels.Snapshot(ch.ID); ok {
			view.SmoothedUpstreamCM = up
			view.SmoothedDownstream = down
			view.HeadDiffCM = head
		}
		for _, g := range a.gates.ListByChamber(ch.ID) {
			view.Gates = append(view.Gates, model.GateView{
				ID:          g.ID,
				Name:        g.Name,
				State:       g.State,
				OpenPercent: g.OpenPercent,
				InPosition:  g.InPosition,
				FaultCode:   g.FaultCode,
				UpdatedAt:   g.UpdatedAt,
			})
		}
		chViews = append(chViews, view)
	}
	denials := journal.MapDeny(a.journal.RecentDenials(10))
	return model.StatusSnapshot{GeneratedAt: now, Chambers: chViews, RecentDeny: denials}
}

func (a *App) updateChamberLevels(chamberID string, now time.Time) {
	up, down, head, ok := a.levels.Snapshot(chamberID)
	if !ok {
		return
	}
	a.chamberMu.Lock()
	defer a.chamberMu.Unlock()
	ch, exists := a.chambers[chamberID]
	if !exists {
		return
	}
	ch.ApplySmoothedLevels(up, down, head, now)
	a.chambers[chamberID] = ch
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func permitHTTPStatus(err error) int {
	if errors.Is(err, permit.ErrTicketExpired) {
		return http.StatusGone
	}
	return http.StatusConflict
}

// IngestLevel is a test helper that applies level samples through the store and snapshot.
func (a *App) IngestLevel(report model.LevelReport) error {
	now := a.now().UTC()
	up, down, head, err := a.levels.Ingest(report, now)
	if err != nil {
		return err
	}
	a.chamberMu.Lock()
	ch, ok := a.chambers[report.ChamberID]
	if ok {
		ch.ApplySmoothedLevels(up, down, head, now)
		a.chambers[report.ChamberID] = ch
	}
	a.chamberMu.Unlock()
	return nil
}

// Gates exposes the gate registry for tests.
func (a *App) Gates() *gatefsm.Registry {
	return a.gates
}

// Permits exposes the permit service for tests.
func (a *App) Permits() *permit.Service {
	return a.permits
}

// Dispatch exposes the PLC client for tests.
func (a *App) Dispatch() *dispatch.PLCClient {
	return a.dispatch
}

// SetDispatch replaces the PLC client (tests only).
func (a *App) SetDispatch(c *dispatch.PLCClient) {
	a.dispatch = c
}

// SetNow replaces the clock function for tests.
func (a *App) SetNow(fn func() time.Time) {
	a.now = fn
}
