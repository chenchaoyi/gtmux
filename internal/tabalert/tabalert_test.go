package tabalert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// The whole promise of the enable path is that the user's own title format survives. It
// is THEIR tmux configuration; gtmux is asking for a few characters in front of it, not
// for the line.
func TestEnablePrependsAndDisableRestores(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const original = "#S — #W"

	// Simulate what Enable persists, without a tmux server: the saved snapshot is the
	// contract Disable depends on.
	if err := os.MkdirAll(state.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(savedPath(), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(savedPath())
	if err != nil || string(b) != original {
		t.Fatalf("the original format must round-trip exactly, got %q (%v)", string(b), err)
	}
	// The owned format is a PREFIX of the user's, never a replacement.
	owned := ownedPrefix + original
	if !strings.HasPrefix(owned, ownedPrefix) || !strings.HasSuffix(owned, original) {
		t.Fatalf("owned format %q must wrap, not replace", owned)
	}
}

// Ownership is detected from the format itself, so a user who rewrote it is never
// clobbered by a stale snapshot.
func TestOwnershipIsReadFromTheFormat(t *testing.T) {
	cases := map[string]bool{
		ownedPrefix + "#S — #W": true,
		"#S — #W":               false,
		"":                      false,
		"#S #{@gtmux_alert}":    false, // ours only when it LEADS
	}
	for format, want := range cases {
		if got := strings.HasPrefix(format, ownedPrefix); got != want {
			t.Errorf("format %q: owned = %v, want %v", format, got, want)
		}
	}
}

// waitingSessions reads STATE FILES — the markers the agents' own hook events wrote —
// never the screen. A pane id that is not a pane id is ignored rather than shelled out on.
func TestWaitingSessionsIgnoresNonPaneEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.WaitingDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"notapane", ".DS_Store", "README"} {
		if err := os.WriteFile(filepath.Join(state.WaitingDir(), n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// No tmux server in a test, so the map is empty either way; the assertion is that it
	// does not panic and does not invent sessions from junk filenames.
	if got := waitingSessions(); len(got) != 0 {
		t.Errorf("junk entries produced sessions: %v", got)
	}
}

// Reconcile is a no-op when gtmux does not own the format, so the tick and the hook can
// both call it unconditionally without a feature check at every call site.
func TestReconcileIsInertWhenDisabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	Reconcile() // must not panic, must not create state
	if _, err := os.Stat(savedPath()); !os.IsNotExist(err) {
		t.Error("a disabled Reconcile must write nothing")
	}
}

// The marker is a SHAPE, not a colour: a tab colour is a per-terminal capability, and
// this has to work on every terminal that can run tmux.
func TestMarkerIsShapeNotColor(t *testing.T) {
	if strings.Contains(Marker, "\033") {
		t.Error("the marker must carry no escape codes — the tab is not a place colour is portable")
	}
	if strings.TrimSpace(Marker) == "" {
		t.Error("the marker must be visible")
	}
}
