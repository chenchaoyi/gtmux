package hook

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// TestDecide is the contract table: (event, active-marker-present?) →
// (state mutations, notify?). Pure logic, no side effects.
func TestDecide(t *testing.T) {
	cases := []struct {
		name          string
		event         string
		activePresent bool
		want          decision
	}{
		{"UserPromptSubmit starts a turn (was idle)", "UserPromptSubmit", false,
			decision{setActive: true, clearWaiting: true, clearFinished: true}},
		{"UserPromptSubmit starts a turn (was active)", "UserPromptSubmit", true,
			decision{setActive: true, clearWaiting: true, clearFinished: true}},
		{"Stop ends a turn, stamps finished, and notifies", "Stop", true,
			decision{clearActive: true, clearWaiting: true, setLastFinished: true, setFinished: true, notify: true}},
		{"Stop while idle still clears + stamps + notifies", "Stop", false,
			decision{clearActive: true, clearWaiting: true, setLastFinished: true, setFinished: true, notify: true}},
		{"Notification mid-turn marks waiting (clears finished)", "Notification", true,
			decision{setWaiting: true, clearFinished: true, setLastFinished: true, notify: true}},
		{"Notification while idle does NOT mark waiting or touch finished", "Notification", false,
			decision{setLastFinished: true, notify: true}},
		{"Resumed clears waiting silently mid-turn", "Resumed", true,
			decision{clearWaiting: true, clearFinished: true}},
		{"Resumed while idle is still just a silent clear", "Resumed", false,
			decision{clearWaiting: true, clearFinished: true}},
		{"StopFailure ends the turn but is NOT a finish (no stamp, no notify)", "StopFailure", true,
			decision{clearActive: true, clearWaiting: true}},
		{"StopFailure while idle still just clears", "StopFailure", false,
			decision{clearActive: true, clearWaiting: true}},
		{"SessionEnd clears the pane's markers silently", "SessionEnd", true,
			decision{clearActive: true, clearWaiting: true, clearFinished: true}},
		{"SessionStart clears the pane's markers silently", "SessionStart", true,
			decision{clearActive: true, clearWaiting: true, clearFinished: true}},
		{"unknown event is a no-op", "Frobnicate", true, decision{}},
		{"empty event is a no-op", "", false, decision{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decide(c.event, c.activePresent); got != c.want {
				t.Errorf("decide(%q, %v) =\n  %+v\nwant\n  %+v", c.event, c.activePresent, got, c.want)
			}
		})
	}
}

// TestExtractEvent: a positional hook arg is either the event name or — for
// Codex's notify — a JSON payload whose "type" is the event.
func TestExtractEvent(t *testing.T) {
	cases := []struct{ in, want string }{
		{`{"type":"agent-turn-complete","turn-id":"x"}`, "agent-turn-complete"},
		{"Stop", "Stop"},
		{"{not valid json", "{not valid json"},
		{`{"foo":1}`, `{"foo":1}`}, // JSON but no "type" → as-is
	}
	for _, c := range cases {
		if got := extractEvent(c.in); got != c.want {
			t.Errorf("extractEvent(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestApplyStateLifecycle walks a realistic turn against a temp HOME and asserts
// the marker files match the contract at each step.
func TestApplyStateLifecycle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	pane := "%7"

	// Prompt submitted → turn in progress.
	applyState(decide("UserPromptSubmit", false), pane)
	if !state.Exists(state.ActivePath(pane)) {
		t.Fatal("UserPromptSubmit should create the active marker")
	}
	if state.Exists(state.WaitingPath(pane)) {
		t.Fatal("UserPromptSubmit should not leave a waiting marker")
	}

	// Notification mid-turn → blocked on the user, and recorded as last-finished.
	active := state.Exists(state.ActivePath(pane))
	applyState(decide("Notification", active), pane)
	if !state.Exists(state.WaitingPath(pane)) {
		t.Error("mid-turn Notification should create the waiting marker")
	}
	if got := state.ReadLastFinished(); got != pane {
		t.Errorf("last-finished = %q, want %q", got, pane)
	}

	// Stop → turn over, both markers cleared, last-finished persists.
	applyState(decide("Stop", true), pane)
	if state.Exists(state.ActivePath(pane)) {
		t.Error("Stop should clear the active marker")
	}
	if state.Exists(state.WaitingPath(pane)) {
		t.Error("Stop should clear the waiting marker")
	}
	if got := state.ReadLastFinished(); got != pane {
		t.Errorf("last-finished after Stop = %q, want %q (must persist)", got, pane)
	}
}

// TestNotificationWhileIdle guards the idle-nudge gotcha: a Notification with no
// active marker is Claude's idle nudge, not a real "blocked on you".
func TestNotificationWhileIdle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	pane := "%9"

	active := state.Exists(state.ActivePath(pane)) // false — no turn in progress
	applyState(decide("Notification", active), pane)

	if state.Exists(state.WaitingPath(pane)) {
		t.Error("idle Notification must NOT create a waiting marker")
	}
	if got := state.ReadLastFinished(); got != pane {
		t.Errorf("idle Notification should still record last-finished; got %q", got)
	}
}

// nativeStateFor maps a canonical lifecycle event to a non-tmux session's state
// (or removal). It mirrors the tmux state transitions, keyed by session instead.
func TestNativeStateFor(t *testing.T) {
	cases := []struct {
		event  string
		wantSt string
		wantRm bool
	}{
		{"UserPromptSubmit", "working", false},
		{"Resumed", "working", false},
		{"Stop", "idle", false},
		{"Waiting", "waiting", false},
		{"Notification", "waiting", false},
		{"SessionStart", "idle", false},
		{"SessionEnd", "", true},
		{"Frobnicate", "", false}, // unknown → no change
	}
	for _, c := range cases {
		st, rm := nativeStateFor(c.event)
		if st != c.wantSt || rm != c.wantRm {
			t.Errorf("nativeStateFor(%q) = (%q,%v), want (%q,%v)", c.event, st, rm, c.wantSt, c.wantRm)
		}
	}
}

// A background row's label is what tells the reader WHAT is running. It used to be the
// command verbatim, and a marker file keeps one line — so a script-shaped command was
// labelled by its first line. Seen on a real row: `prev=""`, the opening assignment of a
// polling loop, sitting where the work should be named.
func TestBackgroundLabelSkipsThePreamble(t *testing.T) {
	for _, tc := range []struct{ name, cmd, want string }{
		{"the real one", "prev=\"\"\nwhile true; do gh pr checks 803; sleep 30; done",
			"while true; do gh pr checks 803; sleep 30; done"},
		{"shell options", "set -e\nnpm run build", "npm run build"},
		{"export", "export PATH=/x\nmake check", "make check"},
		{"comment", "# poll CI\ngh run watch", "gh run watch"},
		{"single line is untouched", "npm test", "npm test"},
		// An env PREFIX is part of the command, not preamble — dropping it would
		// misreport what is running.
		{"env prefix stays", "FOO=1 npm test", "FOO=1 npm test"},
		// Nothing but preamble: say it rather than going blank.
		{"all preamble", "A=1\nB=2", "A=1 B=2"},
	} {
		if got := firstMeaningfulCommand(tc.cmd); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// Nothing in the MIDDLE is dropped: a label that silently omits part of a command is
// worse than a long one, and the caller truncates.
func TestBackgroundLabelKeepsTheWholeCommand(t *testing.T) {
	got := firstMeaningfulCommand("set -e\nmake build\nmake check")
	if got != "make build make check" {
		t.Errorf("got %q — later lines must survive", got)
	}
}

// The WIRING, not just the helper: summarizeBackground must run a command through the
// preamble skip. A pure-function test passes whether or not anything calls it — this one
// fails if summarizeBackground goes back to taking the command verbatim.
func TestSummarizeBackgroundUsesTheFirstRealCommand(t *testing.T) {
	n, label := summarizeBackground([]backgroundTask{
		{Type: "bash", Status: "running", Command: "prev=\"\"\nwhile true; do gh pr checks 803; done"},
	})
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
	if strings.HasPrefix(label, "prev=") {
		t.Errorf("label = %q — the row would say nothing about what is running", label)
	}
	if !strings.Contains(label, "gh pr checks") {
		t.Errorf("label = %q, want the actual command", label)
	}
}
