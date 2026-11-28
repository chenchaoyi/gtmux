package resume

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestCHH008_RetiredFrontendsDoNotExhaustActiveCapAndArePruned(t *testing.T) {
	f, _ := frontendFixture(t)
	f, err := BindCodexFrontend(f, "verified-root")
	if err != nil {
		t.Fatal(err)
	}

	// Seed 160 retired frontend receipts alongside the 1 live frontend.
	started := time.Now().Add(-10 * time.Minute).UnixMilli()
	updated := time.Now().Add(-5 * time.Minute).UnixMilli()
	for n := 1; n <= 160; n++ {
		retired := f
		retired.Instance = fmt.Sprintf("%032x", n)
		retired.Loc = "retired:0.0"
		retired.Retired = true
		retired.StartedAt = started
		retired.UpdatedAt = updated
		b, err := json.Marshal(retired)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(frontendPath(retired.Instance), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// ConversationRecord must ignore retired receipts when checking the 128 active cap.
	Remove(f.Loc)
	rec, recovered, err := ConversationRecord(f.Loc)
	if err != nil || !recovered || rec.SessionID != "verified-root" {
		t.Fatalf("ConversationRecord blocked by >128 retired receipts: rec=%+v recovered=%v err=%v", rec, recovered, err)
	}

	// PruneCodexFrontends removes aged retired receipts while keeping the live frontend intact.
	PruneCodexFrontends(time.Now())
	if _, err := os.Lstat(frontendPath(fmt.Sprintf("%032x", 1))); !os.IsNotExist(err) {
		t.Fatalf("retired frontend receipt remained after PruneCodexFrontends: %v", err)
	}
	if _, err := os.Lstat(frontendPath(f.Instance)); err != nil {
		t.Fatalf("live frontend receipt was removed by PruneCodexFrontends: %v", err)
	}
}
