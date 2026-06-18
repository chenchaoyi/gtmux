package hook

import (
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
			decision{setActive: true, clearWaiting: true}},
		{"UserPromptSubmit starts a turn (was active)", "UserPromptSubmit", true,
			decision{setActive: true, clearWaiting: true}},
		{"Stop ends a turn and notifies", "Stop", true,
			decision{clearActive: true, clearWaiting: true, setLastFinished: true, notify: true}},
		{"Stop while idle still clears + notifies", "Stop", false,
			decision{clearActive: true, clearWaiting: true, setLastFinished: true, notify: true}},
		{"Notification mid-turn marks waiting", "Notification", true,
			decision{setWaiting: true, setLastFinished: true, notify: true}},
		{"Notification while idle does NOT mark waiting", "Notification", false,
			decision{setLastFinished: true, notify: true}},
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

// TestCanonicalEvent: the per-agent translation layer. Claude is identity-mapped
// (so its behavior is unchanged); unknown agents/events are no-ops ("").
func TestCanonicalEvent(t *testing.T) {
	cases := []struct {
		agent, raw, wantEvent, wantDisplay string
	}{
		{"claude", "Stop", "Stop", "Claude Code"},
		{"claude", "UserPromptSubmit", "UserPromptSubmit", "Claude Code"},
		{"claude", "Notification", "Notification", "Claude Code"},
		{"claude", "Frobnicate", "", "Claude Code"},       // unmapped event → no-op event
		{"codex", "agent-turn-complete", "Stop", "Codex"}, // Codex turn done → finished
		{"", "Stop", "", ""},     // no agent
		{"nope", "Stop", "", ""}, // unknown agent → no-op
	}
	for _, c := range cases {
		ev, disp := canonicalEvent(c.agent, c.raw)
		if ev != c.wantEvent || disp != c.wantDisplay {
			t.Errorf("canonicalEvent(%q,%q) = (%q,%q), want (%q,%q)",
				c.agent, c.raw, ev, disp, c.wantEvent, c.wantDisplay)
		}
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
