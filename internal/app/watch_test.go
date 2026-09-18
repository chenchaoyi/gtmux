package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
)

// The live board's colour is the point of it: red is the one state that wants you, and
// for years it was drawn in stRun, the same yellow as running. A reader scanning for the
// row to act on had nothing to scan for.
func TestWaitingIsNotTheColourOfRunning(t *testing.T) {
	waitStyle, _, _ := watchStyle("waiting")
	runStyle, _, _ := watchStyle("running")
	if waitStyle.GetForeground() == runStyle.GetForeground() {
		t.Fatal("waiting and running share a colour — the state that needs you looks like the one that needs nothing")
	}
	if got := waitStyle.GetForeground(); got != lipgloss.Color("#EF4444") {
		t.Errorf("waiting = %v, want DESIGN §9's red #EF4444", got)
	}
}

// Both screens read one glyph table, and the glyphs are text-presentation on purpose: an
// emoji-presentation mark can be drawn from a colour emoji font that ignores the colour
// it was given, putting the red on the word and not on the mark beside it.
func TestTheTwoScreensDrawTheSameGlyphs(t *testing.T) {
	for _, st := range []string{"waiting", "working", "idle", "running"} {
		_, watchG, _ := watchStyle(st)
		listG, _, _ := statusStyle(st)
		if watchG != listG {
			t.Errorf("%s: live board draws %q, the list draws %q", st, watchG, listG)
		}
		if watchG == "" {
			t.Errorf("%s has no glyph", st)
		}
	}
	for _, bad := range []string{"⏸", "✳", "⚠"} {
		for st, g := range agentGlyph {
			if g == bad {
				t.Errorf("%s uses %q, which carries emoji presentation", st, bad)
			}
		}
	}
}

// Each status sits under its own heading, in DESIGN's order. Two statuses sharing one
// heading would put the pane that wants you under the same title as the one that does not.
func TestEachStatusHasItsOwnSection(t *testing.T) {
	defer i18n.SetLang("en")
	i18n.SetLang("en")
	seen := map[string]string{}
	for _, st := range []string{"waiting", "working", "idle", "running"} {
		sec := sectionOf(st)
		if sec == "" {
			t.Errorf("%s has no section", st)
		}
		if other, dup := seen[sec]; dup {
			t.Errorf("%s and %s share the heading %q", other, st, sec)
		}
		seen[sec] = st
	}
}

// The age column reads in seconds while it is seconds. humanize.AgeShort says "just now"
// below a minute, which is right for a board read once an hour and wrong for a screen
// that redraws every 1.5 seconds.
func TestSinceShortCountsSeconds(t *testing.T) {
	for _, c := range []struct {
		secs int64
		want string
	}{{0, ""}, {7, "7s"}, {59, "59s"}, {60, "1m"}, {3600, "1h"}, {3 * 24 * 3600, "3d"}} {
		if got := sinceShort(c.secs); got != c.want {
			t.Errorf("sinceShort(%d) = %q, want %q", c.secs, got, c.want)
		}
	}
}

// Nothing wraps. A wrapped row on a screen that redraws twice a second reads as
// corruption, so the columns give way in order: the agent name first, then the task.
func TestTheBoardFitsTheTerminalItIsGiven(t *testing.T) {
	defer i18n.SetLang("en")
	i18n.SetLang("en")
	// The board reads the plan cache for its footer; give it an empty home so the test
	// exercises the no-data path rather than whoever is running it.
	t.Setenv("HOME", t.TempDir())
	panes := []radar.Pane{
		{PaneID: "%7", Loc: "api:0.0", Agent: "Claude Code", Status: "waiting", Task: "permission to run the whole test suite once more"},
		{PaneID: "%5", Loc: "infra:0.0", Agent: "Claude Code", Status: "running"},
	}
	for _, width := range []int{72, 80, 96, 120} {
		m := watchModel{panes: panes, width: width, finished: map[string]bool{}}
		for _, line := range strings.Split(m.View(), "\n") {
			plain := stripANSI(line)
			if w := i18n.DispWidth(plain); w > width {
				t.Errorf("at %d columns a line is %d wide: %q", width, w, plain)
			}
		}
	}
}
