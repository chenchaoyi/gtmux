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

func TestNewID_Unique(t *testing.T) {
	if NewID(1) == NewID(2) {
		t.Fatalf("distinct timestamps should yield distinct ids")
	}
}
