package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/app"
	"github.com/lacsar712/tidegate/internal/config"
	"github.com/lacsar712/tidegate/internal/ingest"
	"github.com/lacsar712/tidegate/internal/model"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.JournalPath = filepath.Join(dir, "test.journal")
	cfg.ListenAddr = ":0"
	cfg.PLCEndpoint = "http://127.0.0.1:1/unreachable"
	cfg.PLCRetries = 0
	cfg.PLCTimeout = 200 * time.Millisecond
	return cfg
}

func TestAppStatusEndpoint(t *testing.T) {
	cfg := testConfig(t)
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequestOpenDeniedHeadDiff(t *testing.T) {
	cfg := testConfig(t)
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()

	body, _ := json.Marshal(model.OpenRequest{
		ChamberID: cfg.DefaultChamberID,
		GateID:    "gate-upstream",
		Action:    model.ActionOpen,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/ops/request-open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestIngestLevelAndOpenWithMockPLC(t *testing.T) {
	plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer plc.Close()

	cfg := testConfig(t)
	cfg.PLCEndpoint = plc.URL
	cfg.PLCRetries = 1
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()

	fixed := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	application.SetNow(func() time.Time { return fixed })

	if err := application.SeedDemoLevels(cfg.DefaultChamberID, 100, 95); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(model.OpenRequest{
		ChamberID: cfg.DefaultChamberID,
		GateID:    "gate-upstream",
		Action:    model.ActionOpen,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/ops/request-open", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestSensorIngestHMAC(t *testing.T) {
	cfg := testConfig(t)
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()

	verifier, _ := ingest.NewVerifier(cfg.HMACSecret, cfg.SkewSec)
	now := time.Now().UTC()
	payload := model.LevelReport{
		ChamberID: cfg.DefaultChamberID, Probe: model.ProbeUpstream, LevelCM: 100, ReportedAt: now,
	}
	raw, _ := json.Marshal(payload)
	key, ts := verifier.Sign(raw, now)

	req := httptest.NewRequest(http.MethodPost, "/v1/sensors/level", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tide-Key", key)
	req.Header.Set("X-Tide-Time", ts)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("first probe only should wait for pair, got %d", rec.Code)
	}
}

func TestRunFromEnvSmoke(t *testing.T) {
	if os.Getenv("TIDEGATE_INTEGRATION") == "" {
		t.Skip("set TIDEGATE_INTEGRATION=1 to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cfg := testConfig(t)
	cfg.ListenAddr = ":18080"
	application, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = application.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)
}
