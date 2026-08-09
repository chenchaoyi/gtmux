package dispatch

import (
	"os"
	"path/filepath"
	"testing"
)

// Marking puts an entry on the plate and stamps the wait clock; clearing takes it off
// and zeroes the clock. Both directions, because a plate you cannot empty is worse than
// no plate at all.
func TestMarkAndClearAwaitingCommander(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	id := NewID(1000)
	if err := AddTask(Task{ID: id, Pane: "%1", Goal: "cut the release?", CreatedAt: 10}); err != nil {
		t.Fatal(err)
	}
	if got, _ := LoadTask(id); got.AwaitingCommander() {
		t.Fatal("a fresh dispatch must not start on the plate")
	}

	if !MarkAwaitingCommander(id, 500) {
		t.Fatal("mark failed")
	}
	got, _ := LoadTask(id)
	if !got.AwaitingCommander() || got.Disposition != DispositionAwaitingCommander {
		t.Fatalf("not pending after mark: %+v", got)
	}
	if got.AwaitingSince != 500 || got.PendingSince() != 500 {
		t.Fatalf("wait clock = %d/%d, want 500", got.AwaitingSince, got.PendingSince())
	}

	// Re-marking is idempotent and must NOT restart the wait — the view orders by
	// oldest-waiting, so a re-mark would silently move an old item to the back.
	if !MarkAwaitingCommander(id, 900) {
		t.Fatal("re-mark failed")
	}
	if got, _ = LoadTask(id); got.AwaitingSince != 500 {
		t.Errorf("re-mark restarted the wait clock: %d", got.AwaitingSince)
	}

	if !ClearAwaitingCommander(id, "decided", 1000) {
		t.Fatal("clear failed")
	}
	got, _ = LoadTask(id)
	if got.AwaitingCommander() || got.AwaitingSince != 0 {
		t.Fatalf("still pending after clear: %+v", got)
	}
	if got.Disposition != "decided" {
		t.Errorf("resolution not recorded: %q", got.Disposition)
	}
	// The clock falls back to when the entry was first seen, so an unmarked row still
	// sorts sanely if it is ever re-marked.
	if got.PendingSince() != 10 {
		t.Errorf("PendingSince after clear = %d, want the first-seen stamp 10", got.PendingSince())
	}
}

// Any other disposition takes an item off the plate too: "handled" is handled, however
// it was reached. One way in, one way out.
func TestSetDispositionLeavesThePlate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	id := NewID(2000)
	if err := AddTask(Task{ID: id, Pane: "%2", CreatedAt: 20}); err != nil {
		t.Fatal(err)
	}
	if !MarkAwaitingCommander(id, 100) {
		t.Fatal("mark failed")
	}
	if !SetDisposition(id, "relayed", 200) {
		t.Fatal("set failed")
	}
	got, _ := LoadTask(id)
	if got.AwaitingCommander() || got.AwaitingSince != 0 {
		t.Fatalf("a non-awaiting disposition must clear the pending set: %+v", got)
	}
	// And clearing with the awaiting value itself is a resolution, not a no-op re-mark.
	if !MarkAwaitingCommander(id, 300) {
		t.Fatal("re-mark failed")
	}
	if !ClearAwaitingCommander(id, DispositionAwaitingCommander, 400) {
		t.Fatal("clear failed")
	}
	if got, _ = LoadTask(id); got.AwaitingCommander() {
		t.Error("clearing into the awaiting disposition left the item on the plate")
	}
}

// A ledger written by a binary that predates this field must load unchanged — the field
// is ABSENT on disk, not merely empty, and there is real on-disk history to read (never
// migrate). Nothing legacy is pending, and marking one needs no migration.
func TestLegacyLedgerEntryWithoutAwaitingField(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(tasksDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	const legacy = `{
  "id": "t-legacy",
  "pane": "%7",
  "session": "worker",
  "agent": "claude",
  "goal": "an entry from before the plate existed",
  "created_at": 1700000000,
  "delivered": true,
  "tier": "normal",
  "priority": 3,
  "disposition": "relayed",
  "first_seen": 1700000000,
  "last_update": 1700000100
}`
	if err := os.WriteFile(filepath.Join(tasksDir(), "t-legacy.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := LoadTask("t-legacy")
	if !ok {
		t.Fatal("a legacy entry failed to load")
	}
	if got.AwaitingSince != 0 || got.AwaitingCommander() {
		t.Fatalf("legacy entry read as pending: %+v", got)
	}
	if got.Goal != "an entry from before the plate existed" || got.Tier != "normal" || got.Priority != 3 {
		t.Fatalf("legacy fields mangled: %+v", got)
	}
	if got.PendingSince() != 1700000000 {
		t.Errorf("PendingSince = %d, want the first-seen stamp", got.PendingSince())
	}

	// It can join the plate with no migration step.
	if !MarkAwaitingCommander("t-legacy", 1700000500) {
		t.Fatal("marking a legacy entry failed")
	}
	if got, _ = LoadTask("t-legacy"); !got.AwaitingCommander() || got.AwaitingSince != 1700000500 {
		t.Fatalf("legacy entry did not join the plate: %+v", got)
	}
}
