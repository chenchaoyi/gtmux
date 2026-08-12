package hq

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// "Is the pane rendering?" is answered by the HOOK's marker, never by the screen. This
// file predicted both outcomes of the live experiment exactly: the same tty write
// survived on a pane with no marker and was wiped on one that had it.
func TestPaneMidTurnReadsTheHookMarker(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if PaneMidTurn("%9") {
		t.Error("no marker must read as not-mid-turn — an unhooked agent must not queue forever")
	}
	if err := os.MkdirAll(filepath.Dir(state.ActivePath("%9")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := state.Touch(state.ActivePath("%9")); err != nil {
		t.Fatal(err)
	}
	if !PaneMidTurn("%9") {
		t.Error("the hook's active marker means the agent is producing a turn")
	}
	state.Remove(state.ActivePath("%9")) // what Stop does
	if PaneMidTurn("%9") {
		t.Error("Stop clears the marker, so the pane is quiet again")
	}
}

// The spool round-trips a pane id through the filename, because the drain has nothing
// else to target the tty with.
func TestBriefSpoolRoundTripsThePane(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := QueueBrief("%42", "hello"); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(briefDir())
	if err != nil || len(ents) != 1 {
		t.Fatalf("expected one spooled brief, got %v (%v)", len(ents), err)
	}
	if got := briefPaneOf(ents[0].Name()); got != "%42" {
		t.Errorf("pane round-trip = %q, want %%42", got)
	}
	b, _ := os.ReadFile(filepath.Join(briefDir(), ents[0].Name()))
	if string(b) != "hello" {
		t.Errorf("payload = %q", string(b))
	}
}

// Two briefs in one turn are two files: a single-slot spool would silently drop the
// first, and a report that is dropped is worse than one that is late.
func TestBriefSpoolKeepsBoth(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, p := range []string{"a", "b"} {
		if err := QueueBrief("%7", p); err != nil {
			t.Fatal(err)
		}
	}
	ents, _ := os.ReadDir(briefDir())
	if len(ents) != 2 {
		t.Fatalf("expected 2 spooled briefs, got %d", len(ents))
	}
}

// An unparseable name can never be delivered — dropping it keeps the spool from being a
// place things accumulate forever.
func TestBriefPaneOfRejectsJunk(t *testing.T) {
	for _, n := range []string{"", "garbage", "nodash", "pctnodash"} {
		if got := briefPaneOf(n); got != "" {
			t.Errorf("briefPaneOf(%q) = %q, want empty", n, got)
		}
	}
}
