package interlock_test

import (
	"testing"

	"github.com/lacsar712/tidegate/internal/interlock"
	"github.com/lacsar712/tidegate/internal/model"
)

func TestMatrixSymmetric(t *testing.T) {
	m, err := interlock.NewMatrix([]string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetExclusive("a", "b", true); err != nil {
		t.Fatal(err)
	}
	ex, err := m.Exclusive("b", "a")
	if err != nil || !ex {
		t.Fatalf("expected symmetric exclusion, ex=%v err=%v", ex, err)
	}
}

func TestEvaluateConflict(t *testing.T) {
	m, err := interlock.DefaultPairwise([]string{"g1", "g2"})
	if err != nil {
		t.Fatal(err)
	}
	ev := interlock.NewEvaluator(m)
	gates := []model.Gate{
		{ID: "g1", State: model.GateClosed},
		{ID: "g2", State: model.GateOpen},
	}
	allowed, conflict, err := ev.EvaluateRequest("g1", gates, false)
	if err != nil {
		t.Fatal(err)
	}
	if allowed || conflict != "g2" {
		t.Fatalf("expected conflict with g2, allowed=%v conflict=%q", allowed, conflict)
	}
}

func TestBypassSkipsMatrix(t *testing.T) {
	m, _ := interlock.DefaultPairwise([]string{"g1", "g2"})
	ev := interlock.NewEvaluator(m)
	gates := []model.Gate{{ID: "g2", State: model.GateOpen}}
	allowed, _, err := ev.EvaluateRequest("g1", gates, true)
	if err != nil || !allowed {
		t.Fatalf("bypass should allow, allowed=%v err=%v", allowed, err)
	}
}

func TestValidateBypassToken(t *testing.T) {
	if interlock.ValidateBypassToken(true, "secret", "secret") != true {
		t.Fatal("expected valid bypass")
	}
	if interlock.ValidateBypassToken(false, "secret", "secret") {
		t.Fatal("bypass disabled should fail")
	}
}
