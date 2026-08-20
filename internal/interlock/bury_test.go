package interlock_test

import (
	"testing"

	"github.com/lacsar712/tidegate/internal/interlock"
	"github.com/lacsar712/tidegate/internal/model"
)

func TestExclusiveSymmetricDeny(t *testing.T) {
	m, err := interlock.NewMatrix([]string{"up", "down"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetExclusive("up", "down", true); err != nil {
		t.Fatal(err)
	}
	ab, err := m.Exclusive("up", "down")
	if err != nil || !ab {
		t.Fatalf("Exclusive(up,down) want true, got %v err=%v", ab, err)
	}
	ba, err := m.Exclusive("down", "up")
	if err != nil || !ba {
		t.Fatalf("Exclusive(down,up) must also deny (symmetric matrix); got %v err=%v", ba, err)
	}
}

func TestEvaluateSymmetricConflict(t *testing.T) {
	m, err := interlock.NewMatrix([]string{"g1", "g2"})
	if err != nil {
		t.Fatal(err)
	}
	// Set only through SetExclusive — implementation must store both directions.
	if err := m.SetExclusive("g1", "g2", true); err != nil {
		t.Fatal(err)
	}
	ev := interlock.NewEvaluator(m)
	gates := []model.Gate{
		{ID: "g1", State: model.GateOpen},
		{ID: "g2", State: model.GateClosed},
	}
	allowed, conflict, err := ev.EvaluateRequest("g2", gates, false)
	if err != nil {
		t.Fatal(err)
	}
	if allowed || conflict != "g1" {
		t.Fatalf("opening g2 while g1 active must deny symmetrically; allowed=%v conflict=%q", allowed, conflict)
	}
}
