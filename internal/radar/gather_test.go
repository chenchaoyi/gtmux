package radar

import (
	"strconv"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// paneLine builds one tab-separated tmux field-line in the exact order paneSource
// emits and GatherAgents parses: pane_id, session, window, pane, title, command,
// activity_flag, activity, pane_pid, current_path, in_mode.
func paneLine(id, session, window, pane, title, cmd string, activityAt int64, pid int, path string) string {
	return strings.Join([]string{
		id, session, window, pane, title, cmd, "0",
		strconv.FormatInt(activityAt, 10), strconv.Itoa(pid), path, "0",
	}, "\t")
}

// withFixture installs a canned pane source + an empty process table for the span of
// fn, so GatherAgents assembles the fixture rows with no live tmux/ps (the paneSource
// injection seam). The empty procSnapshot keeps the glyph-less-agent subtree path
// deterministic. Everything else (state/resume/transcript/native) reads an empty temp
// HOME the caller sets, so those sources degrade to "".
func withFixture(t *testing.T, lines []string, fn func()) {
	t.Helper()
	origPanes, origProcs := paneSource, procSnapshot
	paneSource = func() []string { return lines }
	procSnapshot = func() map[int]procInfo { return map[int]procInfo{} }
	defer func() { paneSource, procSnapshot = origPanes, origProcs }()
	fn()
}

// TestGatherAgentsFixture drives the full GatherAgents assemble/resolve/sort path over
// injected panes (the coverage lever the paneSource seam unblocks): a spinner pane is
// working, an idle-glyph pane is idle, a pane carrying a fresh waiting marker resolves
// to waiting even over an idle title, a bare shell is excluded, and the rows come back
// sorted needs-you → working → idle.
func TestGatherAgentsFixture(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// A fresh waiting marker for %3 — resolveWaiting must surface it as "waiting"
	// even though the pane title reads as an idle ✳.
	if err := state.WriteMarker(state.WaitingPath("%3"), "permission"); err != nil {
		t.Fatal(err)
	}

	lines := []string{
		paneLine("%1", "work", "0", "0", "✳ finished the task", "claude", 1700000000, 900001, "/tmp/nope"),
		paneLine("%2", "work", "0", "1", "⠋ refactoring auth", "claude", 1700000100, 900002, "/tmp/nope"),
		paneLine("%3", "work", "1", "0", "✳ ready", "claude", 1700000200, 900003, "/tmp/nope"),
		paneLine("%9", "work", "2", "0", "", "bash", 1700000300, 900009, "/tmp/nope"), // bare shell → excluded
	}

	var got []Pane
	withFixture(t, lines, func() { got = GatherAgents() })

	if len(got) != 3 {
		t.Fatalf("want 3 agent rows (the bash pane excluded), got %d: %+v", len(got), got)
	}

	// Sort order: waiting (%3) → working (%2) → idle (%1).
	wantOrder := []struct{ id, status, agent, task string }{
		{"%3", "waiting", "Claude Code", "ready"},
		{"%2", "working", "Claude Code", "refactoring auth"},
		{"%1", "idle", "Claude Code", "finished the task"},
	}
	for i, w := range wantOrder {
		g := got[i]
		if g.PaneID != w.id {
			t.Errorf("row %d: pane = %q, want %q (order waiting→working→idle)", i, g.PaneID, w.id)
		}
		if g.Status != w.status {
			t.Errorf("row %d (%s): status = %q, want %q", i, g.PaneID, g.Status, w.status)
		}
		if g.Agent != w.agent {
			t.Errorf("row %d (%s): agent = %q, want %q", i, g.PaneID, g.Agent, w.agent)
		}
		if g.Task != w.task {
			t.Errorf("row %d (%s): task = %q, want %q", i, g.PaneID, g.Task, w.task)
		}
		if g.Loc == "" || g.source != "tmux" {
			t.Errorf("row %d (%s): loc/source not assembled: loc=%q source=%q", i, g.PaneID, g.Loc, g.source)
		}
	}
}

// TestGatherDigestFixtureLedgerJoin pins the dispatch-ledger join in GatherDigest over
// a fixture radar: a pane tracked by a dispatch task surfaces that task's goal + a
// lifecycle status derived from the pane's radar state; an untracked pane carries none.
func TestGatherDigestFixtureLedgerJoin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Track %2 with a dispatch task; %1 stays untracked.
	id := dispatch.NewID(1000)
	if err := dispatch.AddTask(dispatch.Task{ID: id, Pane: "%2", Goal: "wire up the login flow", CreatedAt: 10}); err != nil {
		t.Fatal(err)
	}

	lines := []string{
		paneLine("%1", "work", "0", "0", "✳ done", "claude", 1700000000, 900001, "/tmp/nope"),
		paneLine("%2", "work", "0", "1", "⠋ working", "claude", 1700000100, 900002, "/tmp/nope"),
	}

	var rows []DigestRow
	withFixture(t, lines, func() { rows = GatherDigest() })

	byPane := map[string]DigestRow{}
	for _, r := range rows {
		byPane[r.PaneID] = r
	}
	tracked, ok := byPane["%2"]
	if !ok {
		t.Fatalf("no digest row for the tracked pane %%2: %+v", rows)
	}
	if tracked.Task != "wire up the login flow" {
		t.Errorf("tracked row task = %q, want the dispatch goal", tracked.Task)
	}
	if tracked.TaskStatus != "working" { // %2 is a spinner → working → ledger "working"
		t.Errorf("tracked row task_status = %q, want %q", tracked.TaskStatus, "working")
	}
	if un := byPane["%1"]; un.Task != "" || un.TaskStatus != "" {
		t.Errorf("untracked pane %%1 should carry no ledger fields, got task=%q status=%q", un.Task, un.TaskStatus)
	}
}
