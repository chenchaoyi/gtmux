package hq

import (
	"reflect"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

// The pane gtmux is running in is not a pane it can type into: it holds the terminal, so
// anything typed is buffered and handed to the agent as stdin when it starts (2026-09-20,
// a startup briefing that arrived as an unsubmitted draft). The self-pane test is what
// routes `--here` and a revive-in-place away from typing.
func TestStartsInOwnPane(t *testing.T) {
	for _, c := range []struct {
		own, pane string
		want      bool
	}{
		{"%20", "%20", true},
		{"%20", "%7", false},
		{"", "%7", false},  // not inside tmux at all: nothing is "our" pane
		{"", "", false},    // and an empty target is never ours
		{"%20", "", false}, // a target we could not resolve is not ours either
	} {
		t.Setenv("TMUX_PANE", c.own)
		if got := startsInOwnPane(c.pane); got != c.want {
			t.Errorf("own=%q pane=%q: startsInOwnPane = %v, want %v", c.own, c.pane, got, c.want)
		}
	}
}

// The detached watcher is told which pane to watch and which agent to expect; the agent
// command travels as ONE argument so a launch command with flags survives.
func TestBriefWatcherArgs(t *testing.T) {
	got := briefWatcherArgs("%20", "claude --model opus")
	want := []string{"hq", "--brief-pane", "%20", "--agent", "claude --model opus"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("briefWatcherArgs = %q, want %q", got, want)
	}
}

// The watcher is off when the briefing is off: GTMUX_HQ_BRIEF is an opt-out of the
// briefing itself, not just of the inline path.
func TestTheWatcherRespectsTheBriefingOptOut(t *testing.T) {
	t.Setenv("GTMUX_HQ_BRIEF", "off")
	if startBriefWatcher("%20", "claude") {
		t.Fatal("a watcher was started while the briefing is opted out")
	}
	if rc := briefPaneWorker("%20", "claude"); rc != 0 {
		t.Fatalf("the worker returned %d with the briefing opted out, want 0 and no delivery", rc)
	}
}

// A queued delivery is a landed one: the agent took the briefing and runs it after the
// turn it is in. Reading it as "not delivered" printed "启动简报未送达" over a pane that
// was already about to brief itself (seen on the real HQ, 2026-09-20).
func TestAQueuedBriefingCounts(t *testing.T) {
	for _, c := range []struct {
		state dispatch.State
		ok    bool
		want  bool
	}{
		{dispatch.StateLanded, true, true},
		{dispatch.StateQueued, false, true},
		{dispatch.StateFailed, false, false},
		{dispatch.StateRefusedDraft, false, false},
	} {
		if got := briefLanded(dispatch.Result{Delivered: c.ok, State: c.state}); got != c.want {
			t.Errorf("briefLanded(%s) = %v, want %v", c.state, got, c.want)
		}
	}
}
