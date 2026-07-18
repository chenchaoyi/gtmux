package app

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

// gatherArchivedTasks surfaces archived ledger entries (status "archived") with the
// attention fields — the data behind `gtmux tasks --verbose`.
func TestGatherArchivedTasks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	id := dispatch.NewID(1000)
	_ = dispatch.AddTask(dispatch.Task{ID: id, Pane: "%1", Goal: "shipped", CreatedAt: 10,
		Tier: "normal", Disposition: "relayed"})
	if !dispatch.ArchiveTask(id, 500) {
		t.Fatal("archive failed")
	}
	rows := gatherArchivedTasks()
	if len(rows) != 1 || rows[0].Status != "archived" || !rows[0].Archived {
		t.Fatalf("archived rows = %+v", rows)
	}
	if rows[0].Tier != "normal" || rows[0].Disposition != "relayed" {
		t.Errorf("attention fields not carried into the row: %+v", rows[0])
	}
}

func TestVerboseTail(t *testing.T) {
	// Without --verbose: no tail regardless of fields.
	if got := verboseTail(taskJSON{Tier: "critical", Priority: 3}, false); got != "" {
		t.Errorf("non-verbose tail should be empty, got %q", got)
	}
	// With --verbose: fields are joined; empties are omitted.
	got := verboseTail(taskJSON{Tier: "critical", Priority: 3, Surfaced: true, Disposition: "relayed"}, true)
	for _, want := range []string{"critical", "p3", "surfaced", "relayed"} {
		if !strings.Contains(got, want) {
			t.Errorf("verbose tail %q missing %q", got, want)
		}
	}
	// Verbose but no fields set → empty (a plain dispatch stays clean).
	if got := verboseTail(taskJSON{}, true); got != "" {
		t.Errorf("verbose tail with no fields should be empty, got %q", got)
	}
}

// A dispatch whose pane the radar surfaced as "waiting" — a stuck-before-running worker
// (startup gate / unsubmitted draft, stuck-dispatch-waiting) — maps to the ledger status
// "waiting", NOT "done". `done` stays reserved for a pane that truly went idle after a
// turn, so HQ is never told a task finished when not one step ran.
func TestTaskStatus_StuckIsWaitingNotDone(t *testing.T) {
	live := map[string]agentPane{
		"%1": {paneID: "%1", status: "waiting"}, // radar flagged it stuck
		"%2": {paneID: "%2", status: "idle"},    // genuinely finished a turn
	}
	if got := taskStatus(dispatch.Task{Pane: "%1"}, live); got != "waiting" {
		t.Errorf("stuck pane task status = %q, want waiting (never done)", got)
	}
	if got := taskStatus(dispatch.Task{Pane: "%2"}, live); got != "done" {
		t.Errorf("genuinely idle pane = %q, want done", got)
	}
}
