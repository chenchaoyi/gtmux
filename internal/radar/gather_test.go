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

// TestGatherAgents_DedupsSharedWindow pins the fix for the "same pane appears twice" bug:
// a window shared across sessions (a tmux session group / linked window — the reported
// "multi-attach" case) makes `list-panes -a` list the SAME pane once PER session, with
// only session/window differing. GatherAgents must emit ONE row per pane_id, not a
// duplicate (which showed a worker under both its own session AND a linked "HQ" one).
func TestGatherAgents_DedupsSharedWindow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	lines := []string{
		// %26: the worker's own session first, then the same pane as seen via a linked
		// "HQ" session — byte-identical except session/window.
		paneLine("%26", "distill-trigger", "1", "0", "⠋ building the trigger", "claude", 1700000100, 900026, "/tmp/wt"),
		paneLine("%26", "HQ", "1", "0", "⠋ building the trigger", "claude", 1700000100, 900026, "/tmp/wt"),
		paneLine("%2", "work", "0", "0", "✳ ready", "claude", 1700000200, 900002, "/tmp/nope"),
	}
	var got []Pane
	withFixture(t, lines, func() { got = GatherAgents() })

	dup := 0
	for _, p := range got {
		if p.PaneID == "%26" {
			dup++
		}
	}
	if dup != 1 {
		t.Fatalf("shared window %%26 must yield ONE row, got %d copies: %+v", dup, got)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows (%%26 deduped + %%2), got %d: %+v", len(got), got)
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

// paneLineHQ is paneLine plus the trailing @gtmux_hq_home stamp field.
func paneLineHQ(id, session, title, cmd, path, stamp string) string {
	return strings.Join([]string{
		id, session, "0", "0", title, cmd, "0", "1700000000", "900100", path, "0", stamp,
	}, "\t")
}

// hq-home-quarantine: when a `gtmux hq`-stamped pane exists, ONLY it carries
// role:"supervisor" — a worker mis-spawned into the HQ home (cwd match) stays a
// normal, visible row instead of masquerading as HQ.
func TestGatherAgents_SupervisorRoleFollowsTheStamp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	hqDir := state.HQHome()

	lines := []string{
		paneLineHQ("%1", "hq", "✳ watching", "claude", hqDir, hqDir),  // the real, stamped HQ
		paneLineHQ("%2", "task", "⠋ building", "claude", hqDir, ""),   // mis-spawned worker in the home
		paneLineHQ("%3", "work", "✳ done", "claude", "/tmp/nope", ""), // normal worker elsewhere
	}
	var got []Pane
	withFixture(t, lines, func() { got = GatherAgents() })

	roles := map[string]string{}
	for _, p := range got {
		roles[p.PaneID] = p.Role()
	}
	if roles["%1"] != "supervisor" {
		t.Fatalf("the stamped pane is the supervisor; roles=%v", roles)
	}
	if roles["%2"] != "" {
		t.Fatalf("a worker parked in the HQ home must stay a NORMAL row; roles=%v", roles)
	}
	if roles["%3"] != "" {
		t.Fatalf("an unrelated pane never gets the role; roles=%v", roles)
	}
}

// Legacy home: no stamp anywhere → the cwd fallback still marks the supervisor.
func TestGatherAgents_CwdFallbackWhenNoStamp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	hqDir := state.HQHome()

	lines := []string{paneLineHQ("%5", "hq", "✳ watching", "claude", hqDir, "")}
	var got []Pane
	withFixture(t, lines, func() { got = GatherAgents() })
	if len(got) != 1 || got[0].Role() != "supervisor" {
		t.Fatalf("cwd fallback must still resolve a legacy HQ; got %+v", got)
	}
}

// tiered-pane-control: a user-watched PLAIN pane appears as a distinct watched radar
// row (no agent status), sorts below agents, and is dropped when its pane closes.
func TestGatherAgents_WatchedPlainPane(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	lines := []string{
		paneLine("%1", "work", "0", "0", "⠋ building", "claude", 1700000000, 900001, "/tmp/x"),
		paneLine("%2", "work", "0", "1", "", "bash", 1700000100, 900002, "/tmp/x"), // plain, watched below
		paneLine("%3", "work", "1", "0", "", "vim", 1700000200, 900003, "/tmp/x"),  // plain, NOT watched
	}
	if err := state.Touch(state.WatchedPath("%2")); err != nil {
		t.Fatal(err)
	}

	var got []Pane
	withFixture(t, lines, func() { got = GatherAgents() })

	byID := map[string]Pane{}
	for _, p := range got {
		byID[p.PaneID] = p
	}
	// %1 the agent is present and NOT watched.
	if _, ok := byID["%1"]; !ok || byID["%1"].Watched {
		t.Fatalf("%%1 should be a normal agent row: %+v", byID["%1"])
	}
	// %2 appears as a watched row with NO agent status.
	w, ok := byID["%2"]
	if !ok || !w.Watched {
		t.Fatalf("%%2 should be a watched row: %+v", w)
	}
	if w.Status != "" {
		t.Fatalf("a watched pane carries no agent status, got %q", w.Status)
	}
	// %3 (unwatched plain vim) must NOT be on the radar.
	if _, ok := byID["%3"]; ok {
		t.Fatal("an unwatched plain pane must not appear on the radar")
	}
	// Watched rows sort below agents.
	lastAgent, firstWatched := -1, len(got)
	for i, p := range got {
		if p.Watched && i < firstWatched {
			firstWatched = i
		}
		if !p.Watched && i > lastAgent {
			lastAgent = i
		}
	}
	if firstWatched < lastAgent {
		t.Fatal("watched rows must sort below all agent rows")
	}
}

// A watched pane whose pane has closed is not rendered (and its marker is reaped).
func TestGatherAgents_WatchedPaneDroppedWhenClosed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// %9 is watched but NOT in the live pane fixture (its pane closed).
	if err := state.Touch(state.WatchedPath("%9")); err != nil {
		t.Fatal(err)
	}
	lines := []string{paneLine("%1", "work", "0", "0", "⠋ go", "claude", 1700000000, 900001, "/tmp/x")}
	var got []Pane
	withFixture(t, lines, func() { got = GatherAgents() })
	for _, p := range got {
		if p.PaneID == "%9" {
			t.Fatal("a watched pane that no longer exists must not be on the radar")
		}
	}
	// The orphan sweep removed its marker.
	if state.IsWatched("%9") {
		t.Fatal("the closed watched pane's marker should be reaped")
	}
}
