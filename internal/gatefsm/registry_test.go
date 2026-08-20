package gatefsm_test

import (
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/gatefsm"
	"github.com/lacsar712/tidegate/internal/model"
)

func TestCanTransitionLegalPath(t *testing.T) {
	steps := []model.GateState{
		model.GateClosed,
		model.GateOpening,
		model.GateOpen,
		model.GateClosing,
		model.GateClosed,
	}
	for i := 0; i < len(steps)-1; i++ {
		if !gatefsm.CanTransition(steps[i], steps[i+1]) {
			t.Fatalf("expected legal transition %s -> %s", steps[i], steps[i+1])
		}
	}
}

func TestIllegalClosedToOpen(t *testing.T) {
	if gatefsm.CanTransition(model.GateClosed, model.GateOpen) {
		t.Fatal("Closed -> Open must be illegal")
	}
}

func TestRegistryApplyReport(t *testing.T) {
	reg := gatefsm.NewRegistry()
	reg.Register(model.Gate{ID: "g1", ChamberID: "c1", Name: "G1", State: model.GateClosed})
	now := time.Now().UTC()
	_, err := reg.ApplyReport(model.GateReport{
		GateID: "g1", ChamberID: "c1", OpenPercent: 10, InPosition: false, ReportedAt: now,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	g, ok := reg.Get("g1")
	if !ok || g.State != model.GateOpening {
		t.Fatalf("expected Opening, got %s", g.State)
	}
}

func TestResetFaultClearsAllFields(t *testing.T) {
	reg := gatefsm.NewRegistry()
	reg.Register(model.Gate{
		ID: "g1", ChamberID: "c1", State: model.GateFault,
		FaultCode: 42, OpenPercent: 55, InPosition: false,
	})
	now := time.Now().UTC()
	g, err := reg.ResetFault("g1", now)
	if err != nil {
		t.Fatal(err)
	}
	if g.State != model.GateClosed {
		t.Fatalf("ResetFault must return Closed state, got %s", g.State)
	}
	if g.FaultCode != 0 || g.OpenPercent != 0 || !g.InPosition {
		t.Fatalf("ResetFault must clear faultCode/openPercent/inPosition; got %+v", g)
	}
}

func TestFaultTransition(t *testing.T) {
	reg := gatefsm.NewRegistry()
	reg.Register(model.Gate{ID: "g1", ChamberID: "c1", State: model.GateOpening})
	now := time.Now().UTC()
	g, err := reg.ApplyReport(model.GateReport{
		GateID: "g1", ChamberID: "c1", FaultCode: 7, ReportedAt: now,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if g.State != model.GateFault {
		t.Fatalf("expected fault, got %s", g.State)
	}
}
