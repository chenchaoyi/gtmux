package radar

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func markTurn(t *testing.T, pane string) {
	t.Helper()
	if err := os.MkdirAll(state.ActiveDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state.ActiveDir(), pane), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The hook knows a turn started and has not ended. That is the fact the status
// should follow — for ANY agent that fires hooks, since the marker is written by
// the shared decision path and carries no agent name.
func TestTurnInProgressFollowsTheMarker(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const now = 1_000_000

	if turnInProgress("%1", now-5, now) {
		t.Error("no marker must not read as a turn")
	}
	markTurn(t, "%1")
	if !turnInProgress("%1", now-5, now) {
		t.Error("a marker on a pane that is still painting means the turn is running")
	}
}

// The polarity is the opposite of the waiting mark's, and getting it backwards would
// break both: an agent blocked on you sits STILL, an agent working repaints. So for a
// turn marker it is activity having STOPPED that contradicts it.
func TestTurnInProgressStopsBelievingAQuietPane(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const now = 1_000_000
	markTurn(t, "%2")

	if !turnInProgress("%2", now-activeQuietGrace+60, now) {
		t.Error("inside the grace the marker still stands")
	}
	if turnInProgress("%2", now-activeQuietGrace-60, now) {
		t.Error("a pane that stopped painting long ago is not still in a turn")
	}
}

// Nothing to contradict the marker with is not a reason to disbelieve it.
func TestTurnInProgressWithNoActivityClock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	markTurn(t, "%3")
	if !turnInProgress("%3", 0, 1_000_000) {
		t.Error("an unknown activity time must not void the marker")
	}
}

// The rule has to reach the ROW, not just be right in isolation. Written after a
// defect injection that disabled the wiring left every unit test green: judging
// correctly and acting on the judgement are two different things.
//
// Deliberately agent-agnostic — the pane below carries Claude's ready glyph and could
// as easily be Codex or opencode, because what decides is the marker, not the name.
func TestGatherAgentsReportsWorkingFromTheTurnMarker(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Real clock: the pipeline stamps `now` from time.Now(), so a fixture activity time
	// must be relative to it — a fake epoch reads as a pane that fell silent decades ago.
	now := time.Now().Unix()

	lines := []string{
		paneLine("%7", "work", "0", "0", "✳ shipping the fix", "2.1.237", now-2, 4242, "/tmp"),
		paneLine("%8", "quiet", "0", "0", "✳ nothing to do", "2.1.237", now-2, 4243, "/tmp"),
	}
	markTurn(t, "%7") // its hook says a turn is running; %8's says nothing

	var got map[string]string
	withFixture(t, lines, func() {
		got = map[string]string{}
		for _, p := range GatherAgents() {
			got[p.PaneID] = p.Status
		}
	})

	if got["%7"] != "working" {
		t.Errorf("a pane whose hook reports a live turn came back %q, want working", got["%7"])
	}
	if got["%8"] != "idle" {
		t.Errorf("a pane with no turn marker came back %q, want idle", got["%8"])
	}
}
