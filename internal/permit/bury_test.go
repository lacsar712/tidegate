package permit_test

import (
	"errors"
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/interlock"
	"github.com/lacsar712/tidegate/internal/level"
	"github.com/lacsar712/tidegate/internal/model"
	"github.com/lacsar712/tidegate/internal/permit"
)

func TestConsumeExpiredUnwraps(t *testing.T) {
	store := level.NewStore(5, 120)
	seedLevels(t, store, "c1")
	m, _ := interlock.NewMatrix([]string{"g1"})
	svc := permit.NewService(store, interlock.NewEvaluator(m), 30, 20*time.Millisecond, false, "")
	now := time.Now().UTC()
	res := svc.EvaluateOpenRequest(model.OpenRequest{ChamberID: "c1", GateID: "g1", Action: model.ActionOpen}, nil, now)
	if !res.Allowed || res.Ticket == nil {
		t.Fatalf("expected ticket, %+v", res)
	}
	_, err := svc.Consume(res.Ticket.ID, now.Add(100*time.Millisecond))
	if err == nil {
		t.Fatal("expected expired consume error")
	}
	if !errors.Is(err, permit.ErrTicketExpired) {
		t.Fatalf("Consume must wrap ErrTicketExpired for errors.Is; got %v", err)
	}
}

func TestValidateTicketExpiredUnwraps(t *testing.T) {
	store := level.NewStore(5, 120)
	seedLevels(t, store, "c1")
	m, _ := interlock.NewMatrix([]string{"g1"})
	svc := permit.NewService(store, interlock.NewEvaluator(m), 30, 20*time.Millisecond, false, "")
	now := time.Now().UTC()
	res := svc.EvaluateOpenRequest(model.OpenRequest{ChamberID: "c1", GateID: "g1", Action: model.ActionOpen}, nil, now)
	_, err := svc.ValidateTicket(res.Ticket.ID, nil, now.Add(100*time.Millisecond))
	if !errors.Is(err, permit.ErrTicketExpired) {
		t.Fatalf("ValidateTicket must wrap ErrTicketExpired for errors.Is; got %v", err)
	}
}

func TestConsumeRemembersNonce(t *testing.T) {
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
	if !svc.NonceSeen(ticket.Nonce) {
		t.Fatalf("Consume must record nonce %q in ledger so replay can be rejected", ticket.Nonce)
	}
	if err := permit.NewNonceLedger(time.Minute).Spend(ticket.Nonce, now); err != nil {
		// fresh ledger — just sanity on Spend API
		_ = err
	}
}

func TestSpendRejectsReplay(t *testing.T) {
	ledger := permit.NewNonceLedger(time.Minute)
	now := time.Now().UTC()
	if err := ledger.Spend("replay-n", now); err != nil {
		t.Fatal(err)
	}
	err := ledger.Spend("replay-n", now)
	if !errors.Is(err, permit.ErrNonceReplay) {
		t.Fatalf("second Spend must reject replay with ErrNonceReplay, got %v", err)
	}
}
