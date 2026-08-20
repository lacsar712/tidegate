package level_test

import (
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/level"
	"github.com/lacsar712/tidegate/internal/model"
	"github.com/lacsar712/tidegate/internal/permit"
	"github.com/lacsar712/tidegate/internal/interlock"
)

func TestExceedsLimitEqualityAllows(t *testing.T) {
	if level.ExceedsLimit(30, 30) {
		t.Fatalf("head diff equal to limit must NOT exceed: ExceedsLimit(30,30)=true (want false so permits at limit remain allowed)")
	}
	if !level.WithinLimit(30, 30) {
		t.Fatal("WithinLimit must accept equality")
	}
}

func TestPermitAllowsHeadAtLimit(t *testing.T) {
	store := level.NewStore(1, 120)
	now := time.Now().UTC()
	_, _, _, err := store.Ingest(model.LevelReport{ChamberID: "c1", Probe: model.ProbeUpstream, LevelCM: 130, ReportedAt: now}, now)
	if err == nil {
		t.Fatal("expected missing downstream")
	}
	_, _, _, err = store.Ingest(model.LevelReport{ChamberID: "c1", Probe: model.ProbeDownstream, LevelCM: 100, ReportedAt: now}, now)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := interlock.NewMatrix([]string{"g1"})
	svc := permit.NewService(store, interlock.NewEvaluator(m), 30, time.Second, false, "")
	res := svc.EvaluateOpenRequest(model.OpenRequest{ChamberID: "c1", GateID: "g1", Action: model.ActionOpen}, nil, now)
	if !res.Allowed {
		t.Fatalf("head diff at exact limit must allow open; deny=%+v", res.Deny)
	}
}

func TestStoreSnapshotUsesMedianNotLast(t *testing.T) {
	store := level.NewStore(5, 120)
	now := time.Now().UTC()
	ch := "c-median"
	// Upstream window: 10,10,10,10,100 → median 10, last 100
	ups := []int{10, 10, 10, 10, 100}
	for i, v := range ups {
		at := now.Add(time.Duration(i) * time.Second)
		_, _, _, err := store.Ingest(model.LevelReport{
			ChamberID: ch, Probe: model.ProbeUpstream, LevelCM: v, ReportedAt: at,
		}, at)
		if i == 0 {
			if err == nil {
				t.Fatal("expected downstream missing on first sample")
			}
			continue
		}
		_, _, _, err = store.Ingest(model.LevelReport{
			ChamberID: ch, Probe: model.ProbeDownstream, LevelCM: 5, ReportedAt: at,
		}, at)
		if err != nil {
			t.Fatal(err)
		}
	}
	up, down, head, ok := store.Snapshot(ch)
	if !ok {
		t.Fatal("snapshot unavailable")
	}
	if up != 10 {
		t.Fatalf("snapshot must use median upstream=10, got up=%d (raw last would be 100) down=%d head=%d", up, down, head)
	}
	if down != 5 || head != 5 {
		t.Fatalf("unexpected downstream/head down=%d head=%d", down, head)
	}
}
