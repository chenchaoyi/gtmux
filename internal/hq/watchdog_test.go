package hq

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/hook"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestShouldEscalate(t *testing.T) {
	const to = int64(600)
	cases := []struct {
		name         string
		status       string
		sinceWait    int64
		now          int64
		alreadyFired bool
		asked        bool
		want         bool
	}{
		{"waiting past timeout, first time", "waiting", 1000, 1000 + 600, false, true, true},
		{"waiting past timeout, already fired", "waiting", 1000, 1000 + 999, true, true, false},
		{"waiting but not long enough", "waiting", 1000, 1000 + 599, false, true, false},
		{"working is never escalated", "working", 1000, 1000 + 9999, false, true, false},
		{"idle is never escalated", "idle", 1000, 1000 + 9999, false, true, false},
		{"no waiting mark", "waiting", 0, 9999, false, true, false},
		// The premise: `stuck·waiting` claims a PERSON IS BLOCKED. If nobody was
		// asked, nobody is stuck — this is the %31 shape (a screen-inferred wait).
		{"nobody asked — a screen-inferred wait", "waiting", 1000, 1000 + 9999, false, false, false},
	}
	for _, c := range cases {
		if got := shouldEscalate(c.status, c.sinceWait, c.now, to, c.alreadyFired, c.asked); got != c.want {
			t.Errorf("%s: shouldEscalate=%v want %v", c.name, got, c.want)
		}
	}
}

// The premise gate, by the kinds that actually reach it — and above all the direction
// that must NOT be lost: a free-form question has no parseable menu (so no `ask`), yet a
// person IS blocked on it. Gating on "empty ask", as the proposal literally specified,
// would have silenced exactly that; gating on PROVENANCE does not.
func TestShouldEscalate_PremiseIsProvenanceNotAskText(t *testing.T) {
	const to, since, now = int64(600), int64(1000), int64(1000 + 700)
	esc := func(kind string) bool {
		return shouldEscalate("waiting", since, now, to, false, hook.IsAskKind(kind))
	}
	// The agent asked. All three escalate — including `question`, whose on-screen form
	// is often free-form prose with no numbered menu at all.
	for _, k := range []string{"permission", "plan", "question"} {
		if !esc(k) {
			t.Errorf("kind %q: the agent asked and nobody answered for 11m — this MUST escalate", k)
		}
	}
	// gtmux talking to itself about a dispatch it read off the screen.
	for _, k := range []string{"startup", "draft", ""} {
		if esc(k) {
			t.Errorf("kind %q: no one was asked, so no one is stuck — must not escalate", k)
		}
	}
}

// The dedup marker fires once per episode and re-arms after removal (what the sweep
// does when a pane leaves waiting).
func TestWatchdogMarker_OncePerEpisode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := watchdogMarker("%1")
	if state.Exists(m) {
		t.Fatal("fresh marker should not exist")
	}
	_ = state.Touch(m)
	if !state.Exists(m) {
		t.Fatal("after Touch the episode is marked (dedup)")
	}
	// Distinct panes have distinct markers.
	if watchdogMarker("%1") == watchdogMarker("%2") {
		t.Fatal("watchdog marker must be per-pane")
	}
	// Leaving waiting removes it → the next episode re-arms.
	state.Remove(m)
	if state.Exists(m) {
		t.Fatal("removal must re-arm the pane for a fresh escalation")
	}
}
