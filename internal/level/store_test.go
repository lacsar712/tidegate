package level_test

import (
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/level"
	"github.com/lacsar712/tidegate/internal/model"
)

func TestMedianInt(t *testing.T) {
	v, ok := level.MedianInt([]int{10, 1, 5})
	if !ok || v != 5 {
		t.Fatalf("median = %d ok=%v", v, ok)
	}
}

func TestHeadDiff(t *testing.T) {
	if level.HeadDiff(120, 100) != 20 {
		t.Fatal("expected 20")
	}
	if level.HeadDiff(90, 110) != 20 {
		t.Fatal("expected absolute diff")
	}
}

func TestStoreIngestMedian(t *testing.T) {
	store := level.NewStore(5, 120)
	now := time.Now().UTC()
	ch := "c1"
	for i := 0; i < 5; i++ {
		_, _, _, err := store.Ingest(model.LevelReport{
			ChamberID: ch, Probe: model.ProbeUpstream, LevelCM: 100 + i%2, ReportedAt: now,
		}, now)
		if i == 0 {
			if err == nil {
				t.Fatal("expected downstream missing error")
			}
			continue
		}
		_, _, _, err = store.Ingest(model.LevelReport{
			ChamberID: ch, Probe: model.ProbeDownstream, LevelCM: 95, ReportedAt: now,
		}, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	up, down, head, ok := store.Snapshot(ch)
	if !ok {
		t.Fatal("snapshot unavailable")
	}
	if up != 100 || down != 95 || head != 5 {
		t.Fatalf("unexpected smoothed values up=%d down=%d head=%d", up, down, head)
	}
}

func TestSkewRejection(t *testing.T) {
	store := level.NewStore(1, 10)
	now := time.Now().UTC()
	old := now.Add(-30 * time.Second)
	_, _, _, err := store.Ingest(model.LevelReport{
		ChamberID: "c1", Probe: model.ProbeUpstream, LevelCM: 100, ReportedAt: old,
	}, now)
	if err == nil {
		t.Fatal("expected skew rejection")
	}
}

func TestWithinLimit(t *testing.T) {
	if !level.WithinLimit(30, 30) {
		t.Fatal("boundary should pass")
	}
	if level.ExceedsLimit(31, 30) != true {
		t.Fatal("31 should exceed 30")
	}
}
