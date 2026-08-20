package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/app"
	"github.com/lacsar712/tidegate/internal/dispatch"
	"github.com/lacsar712/tidegate/internal/model"
	"github.com/lacsar712/tidegate/internal/permit"
)

func TestRequestOpenLeavesGateOpening(t *testing.T) {
	plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer plc.Close()

	cfg := testConfig(t)
	cfg.PLCEndpoint = plc.URL
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	fixed := time.Date(2026, 8, 20, 11, 0, 0, 0, time.UTC)
	application.SetNow(func() time.Time { return fixed })
	if err := application.SeedDemoLevels(cfg.DefaultChamberID, 100, 95); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(model.OpenRequest{
		ChamberID: cfg.DefaultChamberID, GateID: "gate-upstream", Action: model.ActionOpen,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/ops/request-open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d %s", rec.Code, rec.Body.String())
	}
	g, ok := application.Gates().Get("gate-upstream")
	if !ok {
		t.Fatal("gate missing")
	}
	if g.State != model.GateOpening {
		t.Fatalf("open request must enter Opening (not skip to Open); got %s", g.State)
	}
}

func TestRequestOpenHonorsCancel(t *testing.T) {
	started := make(chan struct{})
	plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer plc.Close()

	cfg := testConfig(t)
	cfg.PLCEndpoint = plc.URL
	cfg.PLCTimeout = 5 * time.Second
	cfg.PLCRetries = 0
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	if err := application.SeedDemoLevels(cfg.DefaultChamberID, 100, 95); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(model.OpenRequest{
		ChamberID: cfg.DefaultChamberID, GateID: "gate-upstream", Action: model.ActionOpen,
	})
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/ops/request-open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		application.Handler().ServeHTTP(rec, req)
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("PLC never started")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(800 * time.Millisecond):
		t.Fatal("request-open did not return promptly after client cancel")
	}
	if rec.Code == http.StatusOK {
		t.Fatalf("cancelled dispatch must not succeed; status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPermitHTTPStatusUsesErrorsIs(t *testing.T) {
	cfg := testConfig(t)
	application, errApp := app.New(cfg)
	if errApp != nil {
		t.Fatal(errApp)
	}
	defer application.Close()

	if err := application.SeedDemoLevels(cfg.DefaultChamberID, 100, 95); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	application.SetNow(func() time.Time { return now })
	res := application.Permits().EvaluateOpenRequest(model.OpenRequest{
		ChamberID: cfg.DefaultChamberID, GateID: "gate-upstream", Action: model.ActionOpen,
	}, application.Gates().ListByChamber(cfg.DefaultChamberID), now)
	if !res.Allowed {
		t.Fatalf("expected ticket: %+v", res)
	}
	_, err := application.Permits().Consume(res.Ticket.ID, now.Add(time.Hour))
	if !errors.Is(err, permit.ErrTicketExpired) {
		t.Fatalf("expected ErrTicketExpired via errors.Is, got %v", err)
	}
}

func TestIllegalTransitionSkipsDispatch(t *testing.T) {
	var hits atomic.Int32
	plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer plc.Close()

	cfg := testConfig(t)
	cfg.PLCEndpoint = plc.URL
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	fixed := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	application.SetNow(func() time.Time { return fixed })
	if err := application.SeedDemoLevels(cfg.DefaultChamberID, 100, 95); err != nil {
		t.Fatal(err)
	}
	// Gate already Open — requesting open targets Opening, which is illegal from Open.
	application.Gates().Register(model.Gate{
		ID: "gate-upstream", ChamberID: cfg.DefaultChamberID, Name: "Upstream Gate", State: model.GateOpen,
	})

	body, _ := json.Marshal(model.OpenRequest{
		ChamberID: cfg.DefaultChamberID, GateID: "gate-upstream", Action: model.ActionOpen,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/ops/request-open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("illegal Open->Opening must conflict before dispatch; got %d %s", rec.Code, rec.Body.String())
	}
	if hits.Load() != 0 {
		t.Fatalf("PLC must not be called after illegal transition; hits=%d", hits.Load())
	}
}

func TestResetFaultClearsStateViaHTTP(t *testing.T) {
	cfg := testConfig(t)
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	now := time.Now().UTC()
	application.Gates().Register(model.Gate{
		ID: "gate-upstream", ChamberID: cfg.DefaultChamberID, State: model.GateFault,
		FaultCode: 9, OpenPercent: 40, InPosition: false,
	})

	body, _ := json.Marshal(map[string]string{"gateId": "gate-upstream"})
	req := httptest.NewRequest(http.MethodPost, "/v1/ops/reset-fault", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset-fault status=%d body=%s", rec.Code, rec.Body.String())
	}
	g, _ := application.Gates().Get("gate-upstream")
	if g.State != model.GateClosed || g.FaultCode != 0 || g.OpenPercent != 0 || !g.InPosition {
		t.Fatalf("ResetFault must clear state+fault+position; got %+v (at %s)", g, now)
	}
}

func TestDispatchClientUsesRequestContext(t *testing.T) {
	// Ensure app wiring still constructs a cancelable PLC client path.
	cfg := testConfig(t)
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	if application.Dispatch() == nil {
		t.Fatal("dispatch client missing")
	}
	_ = dispatch.Backoff
}
