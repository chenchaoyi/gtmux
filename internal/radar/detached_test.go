package radar

import (
	"strconv"
	"strings"
	"testing"
)

// A pane line WITH the trailing session_attached field. paneLine (gather_test) predates
// it and stops at in_mode — deliberately kept that way, because a short line is exactly
// what an older tmux or an older fixture produces and the parser must survive it.
func paneLineAttached(id, session, title, cmd string, attached bool) string {
	return strings.Join([]string{
		id, session, "0", "0", title, cmd, "0",
		strconv.FormatInt(0, 10), "0", "/tmp", "0", "", map[bool]string{true: "1", false: "0"}[attached],
	}, "\t")
}

// A row nobody can jump to must SAY so. Measured on a real fleet: the session
// `disk-drift-triage` (spawned --headless, which opens no tab) sat at
// `session_attached=0`, and clicking its row did nothing at all — `focus` selected the
// pane inside tmux and then looked for a terminal tab that could not exist.
func TestDetachedIsReportedFromTheClientCount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	lines := []string{
		paneLineAttached("%1", "shown", "✳ working on it", "claude", true),
		paneLineAttached("%2", "background", "✳ working on it", "claude", false),
	}
	withFixture(t, lines, func() {
		by := map[string]bool{}
		for _, a := range GatherAgents() {
			by[a.PaneID] = a.detached
		}
		if by["%1"] {
			t.Error("%1 has a client attached — it must not be marked detached")
		}
		if !by["%2"] {
			t.Error("%2 has no client attached: nothing on screen shows it, and the row must say so")
		}
	})
}

// A line that predates the field must read as ORDINARY, never as detached. Absent data
// cannot be allowed to invent a warning — the radar would mark every row on a tmux too
// old to answer, which is worse than not marking any.
func TestAShortLineIsNotDetached(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := paneLine("%9", "old", "0", "0", "✳ working on it", "claude", 0, 0, "/tmp")
	withFixture(t, []string{old}, func() {
		rows := GatherAgents()
		if len(rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(rows))
		}
		if rows[0].detached {
			t.Error("a line with no session_attached field must not report detached")
		}
	})
}
