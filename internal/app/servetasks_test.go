package app

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

// The join the phone's background-task row rests on (chat-background-tasks): the ledger
// says what was sent, the radar says what that pane is doing now.
func TestJoinTasksCarriesTheLivePaneStatus(t *testing.T) {
	tasks := []dispatch.Task{
		{ID: "t1", Pane: "%21", Agent: "claude", Goal: "  add --check to restore  ", CreatedAt: 1000},
		{ID: "t2", Pane: "%33", Agent: "codex", Goal: "weight the KB search", CreatedAt: 1100},
	}
	live := map[string]string{"%21": "waiting", "%33": "working"}
	got := joinTasks(tasks, live)
	if len(got) != 2 {
		t.Fatalf("got %d tasks, want 2", len(got))
	}
	if got[0].Status != "waiting" {
		t.Errorf("t1 status = %q, want waiting", got[0].Status)
	}
	if got[0].Goal != "add --check to restore" {
		t.Errorf("the goal kept its padding: %q", got[0].Goal)
	}
	if got[1].Status != "working" {
		t.Errorf("t2 status = %q, want working", got[1].Status)
	}
}

// A task whose pane is gone is reported as gone, never dropped: it happened, and a list
// that silently loses it reads as though it never did.
func TestAGonePaneIsReportedNotDropped(t *testing.T) {
	tasks := []dispatch.Task{
		{ID: "t1", Pane: "%21", Goal: "still going", CreatedAt: 1000},
		{ID: "t2", Pane: "%44", Goal: "its pane was closed", CreatedAt: 900},
		{ID: "t3", Pane: "", Goal: "never had a pane", CreatedAt: 800},
	}
	got := joinTasks(tasks, map[string]string{"%21": "working"})
	if len(got) != 3 {
		t.Fatalf("got %d tasks, want all 3 kept", len(got))
	}
	for _, want := range []struct {
		id, status string
	}{{"t1", "working"}, {"t2", "gone"}, {"t3", "gone"}} {
		var found string
		for _, g := range got {
			if g.ID == want.id {
				found = g.Status
			}
		}
		if found != want.status {
			t.Errorf("%s status = %q, want %q", want.id, found, want.status)
		}
	}
}

// An entry written before CreatedAt existed still has to say when it started.
func TestTaskSinceFallsBackToFirstSeen(t *testing.T) {
	if got := taskSince(dispatch.Task{CreatedAt: 1000, FirstSeen: 500}); got != 1000 {
		t.Errorf("with CreatedAt set, since = %d, want 1000", got)
	}
	if got := taskSince(dispatch.Task{FirstSeen: 500}); got != 500 {
		t.Errorf("without CreatedAt, since = %d, want the first-seen stamp", got)
	}
}
