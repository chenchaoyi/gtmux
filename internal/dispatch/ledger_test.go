package dispatch

import "testing"

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

	// An archived entry is closed business.
	t.Setenv("HOME", t.TempDir())
	_ = AddTask(Task{ID: NewID(8000), Pane: "%8", Session: "fix-auth", CreatedAt: 80,
		OwnSession: true, Archived: true})
	if _, ok := ResumableTask("", "fix-auth"); ok {
		t.Error("an archived entry must not resume")
	}
}

func TestNewID_Unique(t *testing.T) {
	if NewID(1) == NewID(2) {
		t.Fatalf("distinct timestamps should yield distinct ids")
	}
}
