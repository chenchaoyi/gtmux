package dispatch

import (
	"strings"
	"testing"
)

func TestLedger_RoundTripAndList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	a := Task{ID: NewID(1000), Pane: "%1", Goal: "build", CreatedAt: 10, Delivered: true}
	b := Task{ID: NewID(2000), Pane: "%2", Goal: "test", CreatedAt: 20, Worktree: "/wt/x", Branch: "feat/x"}
	if err := AddTask(a); err != nil {
		t.Fatal(err)
	}
	if err := AddTask(b); err != nil {
		t.Fatal(err)
	}

	got, ok := LoadTask(a.ID)
	if !ok || got.Goal != "build" || !got.Delivered {
		t.Fatalf("load a: %+v ok=%v", got, ok)
	}

	list := ListTasks()
	if len(list) != 2 || list[0].ID != b.ID { // newest first
		t.Fatalf("list order wrong: %+v", list)
	}

	if tp, ok := TaskForPane("%2"); !ok || tp.Branch != "feat/x" {
		t.Fatalf("TaskForPane: %+v ok=%v", tp, ok)
	}

	RemoveTask(a.ID)
	if _, ok := LoadTask(a.ID); ok {
		t.Fatalf("removed task still loads")
	}
}

func TestLedger_Snooze(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	task := Task{ID: NewID(3000), Pane: "%3", Goal: "x", CreatedAt: 30}
	_ = AddTask(task)

	if task.Snoozed(100) {
		t.Fatalf("fresh task should not be snoozed")
	}
	if !SnoozeTask(task.ID, 500) {
		t.Fatalf("snooze should succeed")
	}
	got, _ := LoadTask(task.ID)
	if !got.Snoozed(400) {
		t.Fatalf("should be snoozed before the deadline")
	}
	if got.Snoozed(600) {
		t.Fatalf("should not be snoozed after the deadline")
	}
	if SnoozeTask("nonexistent", 500) {
		t.Fatalf("snoozing a missing task must be a no-op false")
	}
}

func TestLedger_Source(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// A stamped source round-trips.
	a := Task{ID: NewID(4000), Pane: "%4", Goal: "x", CreatedAt: 40, Source: SourceUserDirect}
	if err := AddTask(a); err != nil {
		t.Fatal(err)
	}
	got, _ := LoadTask(a.ID)
	if got.Source != SourceUserDirect || got.SourceOrDefault() != SourceUserDirect {
		t.Fatalf("source round-trip: %q / %q", got.Source, got.SourceOrDefault())
	}
	// A legacy entry (no source) defaults to hq-dispatched.
	b := Task{ID: NewID(5000), Pane: "%5", Goal: "y", CreatedAt: 50}
	if err := AddTask(b); err != nil {
		t.Fatal(err)
	}
	gb, _ := LoadTask(b.ID)
	if gb.Source != "" || gb.SourceOrDefault() != SourceHQDispatched {
		t.Fatalf("legacy default: source=%q effective=%q", gb.Source, gb.SourceOrDefault())
	}
}

// ResumableTask is what stops a retry from parking a SECOND empty session beside the
// first. It must find only a previous attempt at THIS dispatch that never delivered its
// goal — and must not reach for a landed dispatch, a borrowed pane, or someone else's
// worktree.
func TestResumableTask(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	landed := Task{ID: NewID(1000), Pane: "%1", Session: "s", Worktree: "/wt/x",
		CreatedAt: 10, Delivered: true, OwnSession: true}
	borrowed := Task{ID: NewID(2000), Pane: "%2", Session: "s", Worktree: "/wt/x",
		CreatedAt: 20, Delivered: false, OwnSession: false} // --pane reuse: nothing spawn owns
	other := Task{ID: NewID(3000), Pane: "%3", Session: "s", Worktree: "/wt/other",
		CreatedAt: 30, Delivered: false, OwnSession: true}
	old := Task{ID: NewID(4000), Pane: "%4", Session: "s", Worktree: "/wt/x",
		CreatedAt: 40, Delivered: false, OwnSession: true}
	newest := Task{ID: NewID(5000), Pane: "%5", Session: "s", Worktree: "/wt/x",
		CreatedAt: 50, Delivered: false, OwnSession: true}
	for _, task := range []Task{landed, borrowed, other, old, newest} {
		if err := AddTask(task); err != nil {
			t.Fatal(err)
		}
	}

	got, ok := ResumableTask("/wt/x", "")
	if !ok || got.ID != newest.ID {
		t.Fatalf("ResumableTask(/wt/x) = %+v ok=%v, want the newest undelivered own-session entry (%s)", got, ok, newest.ID)
	}
	if _, ok := ResumableTask("/wt/nothing", ""); ok {
		t.Error("an unrelated worktree must not resume")
	}

	// With no worktree, the derived session NAME is the key — and it must not match an
	// entry that belongs to a worktree dispatch.
	t.Setenv("HOME", t.TempDir())
	plain := Task{ID: NewID(6000), Pane: "%6", Session: "fix-auth", CreatedAt: 60, OwnSession: true}
	wtSame := Task{ID: NewID(7000), Pane: "%7", Session: "fix-auth", Worktree: "/wt/y",
		CreatedAt: 70, OwnSession: true}
	_ = AddTask(plain)
	_ = AddTask(wtSame)
	got, ok = ResumableTask("", "fix-auth")
	if !ok || got.ID != plain.ID {
		t.Fatalf("ResumableTask(\"\", fix-auth) = %+v ok=%v, want the worktree-less entry", got, ok)
	}
	if _, ok := ResumableTask("", "some-other-session"); ok {
		t.Error("a different session name must not resume")
	}

}

func TestNewID_Unique(t *testing.T) {
	if NewID(1) == NewID(2) {
		t.Fatalf("distinct timestamps should yield distinct ids")
	}
}

// Undelivered is the fact every status view must respect, because the pane cannot
// report it: a dispatch that dies at the ready gate leaves a live, EMPTY, idle agent
// pane that looks exactly like one which just finished a turn.
func TestUndelivered(t *testing.T) {
	cases := []struct {
		name string
		t    Task
		want bool
	}{
		{"landed", Task{Delivered: true, State: string(StateLanded)}, false},
		{"legacy landed (no state)", Task{Delivered: true}, false},
		{"ready-timeout", Task{Delivered: false, State: string(StateFailed)}, true},
		{"legacy failure (no state)", Task{Delivered: false}, true},
		{"refused draft", Task{Delivered: false, State: string(StateRefusedDraft)}, true},
		// Queued reached the agent — it simply runs after the current turn.
		{"queued", Task{Delivered: false, State: string(StateQueued)}, false},
	}
	for _, c := range cases {
		if got := c.t.Undelivered(); got != c.want {
			t.Errorf("%s: Undelivered() = %v, want %v", c.name, got, c.want)
		}
	}
}

// MarkDelivered closes the loop on the documented RESCUE: a failed spawn's goal
// re-sent into the pane it left behind. Without it the rescue works and the ledger row
// says `undelivered` forever while the worker is busy doing the job.
func TestMarkDelivered_Rescue(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	const goal = "implement the readiness fix and open a PR"
	id := NewID(1000)
	_ = AddTask(Task{ID: id, Pane: "%5", Goal: goal, CreatedAt: 10,
		Delivered: false, State: string(StateFailed)})

	// An UNRELATED send into that pane must not launder a dispatch that never happened —
	// the whole value of `undelivered` is that the ledger tells the truth.
	if MarkDelivered("%5", "keep going", 98) {
		t.Error("an unrelated send must not record the dispatch as delivered")
	}
	// The rescue: the same goal, re-sent (line breaks collapsed, as the ledger stores it).
	if !MarkDelivered("%5", "implement the readiness fix\nand open a PR", 99) {
		t.Fatal("a rescue into an undelivered entry's pane must record the landing")
	}
	got, ok := LoadTask(id)
	if !ok || !got.Delivered || got.State != string(StateLanded) || got.Undelivered() {
		t.Fatalf("after the rescue: %+v", got)
	}
	if got.LastUpdate != 99 {
		t.Errorf("LastUpdate = %d, want the passed clock", got.LastUpdate)
	}
	// Idempotent, and never invents an entry for an untracked pane.
	if MarkDelivered("%5", goal, 100) {
		t.Error("an already-landed entry must not be rewritten")
	}
	if MarkDelivered("%404", goal, 100) {
		t.Error("a pane with no ledger entry must be a no-op")
	}
}

// The stored goal is truncated (radar.Snip 200) and whitespace-collapsed, so the match
// is head-only over the same normalization — an equality check could never succeed for a
// long goal, which is exactly the kind a --goal-file dispatch carries.
func TestDeliversGoal(t *testing.T) {
	long := strings.Repeat("把 hqPlaybookVersion 提到 13 并且运行 make check 然后开 PR ", 6)
	// What radar.Snip(long, 200) leaves behind: whitespace collapsed, cut at 200 RUNES.
	stored := string([]rune(collapseSpace(long))[:200]) + "…"
	if !deliversGoal(stored, long) {
		t.Error("the full goal must match its own truncated, stored head")
	}
	if deliversGoal(stored, "别的事情") {
		t.Error("a different message must not match")
	}
	if deliversGoal("", "anything") {
		t.Error("an entry with no recorded goal has nothing to match")
	}
}
