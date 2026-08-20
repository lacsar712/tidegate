package ingest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/gatefsm"
	"github.com/lacsar712/tidegate/internal/journal"
	"github.com/lacsar712/tidegate/internal/level"
)

func TestVerifyEmptySecretNoPanic(t *testing.T) {
	v := &Verifier{secret: "", skewSec: 60}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Verify must not panic on empty secret, recovered=%v", r)
		}
	}()
	err := v.Verify(map[string]string{
		headerKey:  "deadbeef",
		headerTime: "1710000000",
	}, []byte(`{}`), time.Now().UTC())
	if err == nil {
		t.Fatal("expected error for empty HMAC secret")
	}
	if !strings.Contains(err.Error(), "secret") {
		t.Fatalf("expected secret error, got %v", err)
	}
}

func TestHandlerEmptySecretUnauthorized(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.NewJournal(dir+"/j", 10)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	h := NewHandler(&Verifier{secret: "", skewSec: 60}, level.NewStore(3, 120), gatefsm.NewRegistry(), j, 100)
	mux := http.NewServeMux()
	h.Mount(mux)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler must not panic on empty secret, recovered=%v", r)
		}
	}()
	req := httptest.NewRequest(http.MethodPost, "/v1/sensors/level", strings.NewReader(`{}`))
	req.Header.Set("X-Tide-Key", "x")
	req.Header.Set("X-Tide-Time", "1710000000")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty secret ingest must return 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}
