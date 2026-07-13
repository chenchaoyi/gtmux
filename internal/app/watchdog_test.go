package app

import (
	"testing"

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
		want         bool
	}{
		{"waiting past timeout, first time", "waiting", 1000, 1000 + 600, false, true},
		{"waiting past timeout, already fired", "waiting", 1000, 1000 + 999, true, false},
		{"waiting but not long enough", "waiting", 1000, 1000 + 599, false, false},
		{"working is never escalated", "working", 1000, 1000 + 9999, false, false},
		{"idle is never escalated", "idle", 1000, 1000 + 9999, false, false},
		{"no waiting mark", "waiting", 0, 9999, false, false},
	}
	for _, c := range cases {
		if got := shouldEscalate(c.status, c.sinceWait, c.now, to, c.alreadyFired); got != c.want {
			t.Errorf("%s: shouldEscalate=%v want %v", c.name, got, c.want)
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
