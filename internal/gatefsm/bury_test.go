package gatefsm_test

import (
	"strings"
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/gatefsm"
	"github.com/lacsar712/tidegate/internal/model"
)

func TestRequestTransitionRejectsClosedToOpen(t *testing.T) {
	reg := gatefsm.NewRegistry()
	reg.Register(model.Gate{ID: "g1", ChamberID: "c1", State: model.GateClosed})
	now := time.Now().UTC()
	_, err := reg.RequestTransition("g1", model.GateOpen, now)
	if err == nil {
		t.Fatalf("Closed -> Open must be rejected by RequestTransition; want illegal transition error, gate stayed Closed")
	}
	if !strings.Contains(err.Error(), "illegal gate transition") {
		t.Fatalf("expected illegal transition detail in error, got %v", err)
	}
	g, _ := reg.Get("g1")
	if g.State != model.GateClosed {
		t.Fatalf("gate state should remain Closed after rejected skip, got %s", g.State)
	}
}
