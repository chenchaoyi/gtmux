package hq

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// The sweep is throttled, not per-tick: reconciliation has work only when panes close.
// The gate must also be honest about WHEN it last ran, or a serve restart would turn the
// throttle off.
func TestDeadPaneSweepIsThrottled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(state.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	now := int64(1_800_000_000)

	// First call runs (with no tmux in a test env it reaps nothing) and stamps the clock.
	deadPaneSweep(now)
	if got := state.ReadInt64Marker(deadPaneSweepLastPath()); got != now {
		t.Fatalf("sweep clock = %d, want %d — an unstamped sweep runs on every tick", got, now)
	}
	// A tick moments later must not re-run: the marker stays at the first stamp.
	deadPaneSweep(now + 30)
	if got := state.ReadInt64Marker(deadPaneSweepLastPath()); got != now {
		t.Errorf("sweep clock moved to %d within the interval; want it held at %d", got, now)
	}
	// Past the interval it runs again.
	deadPaneSweep(now + deadPaneSweepInterval + 1)
	if got := state.ReadInt64Marker(deadPaneSweepLastPath()); got != now+deadPaneSweepInterval+1 {
		t.Errorf("sweep clock = %d; want it to advance once the interval elapsed", got)
	}
}

// The throttle marker is a plain state file, not something that can be mistaken for a
// pane-keyed record (which the sweep itself would then delete).
func TestDeadPaneSweepMarkerIsNotPaneKeyed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if state.IsPaneID(filepath.Base(deadPaneSweepLastPath())) {
		t.Fatal("the sweep's own clock must not look like a pane record")
	}
}
