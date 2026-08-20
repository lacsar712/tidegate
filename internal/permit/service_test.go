package permit_test

import (
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/interlock"
	"github.com/lacsar712/tidegate/internal/level"
	"github.com/lacsar712/tidegate/internal/model"
	"github.com/lacsar712/tidegate/internal/permit"
)

func seedLevels(t *testing.T, store *level.Store, chamber string) {
	t.Helper()
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		_, _, _, err := store.Ingest(model.LevelReport{ChamberID: chamber, Probe: model.ProbeUpstream, LevelCM: 100, ReportedAt: now}, now)
		if err != nil && i > 0 {
			t.Fatal(err)
		}
		_, _, _, err = store.Ingest(model.LevelReport{ChamberID: chamber, Probe: model.ProbeDownstream, LevelCM: 95, ReportedAt: now}, now)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestPermitHeadDiffDeny(t *testing.T) {
	store := level.NewStore(5, 120)
	seedLevels(t, store, "c1")
	m, _ := interlock.NewMatrix([]string{"g1"})
	svc := permit.NewService(store, interlock.NewEvaluator(m), 3, 15*time.Second, false, "")
	res := svc.EvaluateOpenRequest(model.OpenRequest{ChamberID: "c1", GateID: "g1", Action: model.ActionOpen}, nil, time.Now().UTC())
	if res.Allowed || res.Deny == nil || res.Deny.Code != "HEAD_DIFF" {
		t.Fatalf("expected head diff denial, %+v", res)
	}
}

func TestPermitIssueAndExpire(t *testing.T) {
	store := level.NewStore(5, 120)
	seedLevels(t, store, "c1")
	m, _ := interlock.NewMatrix([]string{"g1"})
	svc := permit.NewService(store, interlock.NewEvaluator(m), 30, 50*time.Millisecond, false, "")
	now := time.Now().UTC()
	res := svc.EvaluateOpenRequest(model.OpenRequest{ChamberID: "c1", GateID: "g1", Action: model.ActionOpen}, nil, now)
	if !res.Allowed || res.Ticket == nil {
		t.Fatalf("expected ticket, %+v", res)
	}
	_, err := svc.ValidateTicket(res.Ticket.ID, nil, now.Add(100*time.Millisecond))
	if err == nil {
		t.Fatal("expected expired ticket")
	}
}

func TestNonceLedger(t *testing.T) {
	ledger := permit.NewNonceLedger(time.Minute)
	now := time.Now().UTC()
	if err := ledger.Spend("n1", now); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Spend("n1", now); err != permit.ErrNonceReplay {
		t.Fatalf("expected replay error, got %v", err)
	}
}

func TestConsumeTicket(t *testing.T) {
	store := level.NewStore(5, 120)
	seedLevels(t, store, "c1")
	m, _ := interlock.NewMatrix([]string{"g1"})
	svc := permit.NewService(store, interlock.NewEvaluator(m), 30, time.Second, false, "")
	now := time.Now().UTC()
	res := svc.EvaluateOpenRequest(model.OpenRequest{ChamberID: "c1", GateID: "g1", Action: model.ActionOpen}, nil, now)
	ticket, err := svc.Consume(res.Ticket.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if !ticket.Used {
		t.Fatal("ticket should be marked used")
	}
	_, err = svc.Consume(res.Ticket.ID, now)
	if err == nil {
		t.Fatal("expected duplicate consume failure")
	}
}
