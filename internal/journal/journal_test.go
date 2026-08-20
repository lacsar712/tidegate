package journal_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/journal"
)

func TestJournalAppendAndRecent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "j.log")
	j, err := journal.NewJournal(path, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		if err := j.Append(journal.Entry{Kind: journal.KindDeny, Message: "deny", At: now}); err != nil {
			t.Fatal(err)
		}
	}
	if len(j.RecentDenials(2)) != 2 {
		t.Fatal("expected 2 recent denials")
	}
}

func TestMapDeny(t *testing.T) {
	now := time.Now().UTC()
	out := journal.MapDeny([]journal.Entry{{Kind: journal.KindDeny, Message: "x", At: now}})
	if len(out) != 1 || out[0].Message != "x" {
		t.Fatalf("unexpected map %+v", out)
	}
}
