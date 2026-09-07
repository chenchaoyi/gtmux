package panefocus

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// This package had no tests at all — the only package in the repo at 0% — while owning
// the jump that the menu bar, the phone, the web mirror and `gtmux focus` all perform.
// What is testable without a tmux server is the part that matters most: the input guard
// on a REMOTE-reachable entry point, and the promise that every path degrades quietly
// when there is no tmux rather than panicking.

func TestFocusPaneByIDRejectsAnythingThatIsNotAPaneID(t *testing.T) {
	// The id comes from POST /api/focus, so it is remote input. It is validated BEFORE
	// tmux is consulted, which is why these answers do not depend on a running server.
	bad := []string{"", "12", "%", "%%1", "pane1", "%12x", "%12 %13", "%12; tmux kill-server", "%-1", " %12"}
	for _, id := range bad {
		err := FocusPaneByID(id)
		if err == nil {
			t.Errorf("FocusPaneByID(%q) = nil, want an error", id)
			continue
		}
		if !strings.Contains(err.Error(), "not a pane id") {
			t.Errorf("FocusPaneByID(%q) failed with %q — the caller is told about the wrong problem", id, err)
		}
	}
}

func TestFocusPaneByIDAcceptsARealPaneIDAndThenAsksTmux(t *testing.T) {
	// A well-formed id passes validation and the answer comes from tmux. With no tmux on
	// this machine the honest answer is that the pane is not there — never "not a pane id".
	old := tmux.Bin
	tmux.Bin = ""
	defer func() { tmux.Bin = old }()
	for _, id := range []string{"%0", "%12", "%99999"} {
		err := FocusPaneByID(id)
		if err == nil {
			t.Fatalf("FocusPaneByID(%q) = nil with no tmux", id)
		}
		if strings.Contains(err.Error(), "not a pane id") {
			t.Errorf("FocusPaneByID(%q) blamed the id: %q", id, err)
		}
	}
}

func TestNothingPanicsWithoutTmux(t *testing.T) {
	// Every surface calls these; a machine without tmux (or with the server stopped) must
	// get a quiet no, not a crash in the menu bar's poll.
	old := tmux.Bin
	tmux.Bin = ""
	defer func() { tmux.Bin = old }()
	JumpPane("%12") // no panic, no output
	if Attached("some-session") {
		t.Error("Attached said yes with no tmux to ask")
	}
	if Attached("") {
		t.Error("Attached said yes for an empty session name")
	}
}
