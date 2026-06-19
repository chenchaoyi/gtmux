package state

import "testing"

// TestFrameWorking is the contract table for the working-vs-idle decision: given
// the previous record + current frame, is the pane animating (working)?
func TestFrameWorking(t *testing.T) {
	const now = 1000
	cases := []struct {
		name                 string
		prevPoll, prevChange int64
		prevHash, curHash    string
		wantChange           int64
		wantWorking          bool
	}{
		{"first sight (no baseline) → idle", 0, 0, "", "a", 0, false},
		{"stale baseline (last poll too old) → idle", now - 30, now - 1, "a", "b", 0, false},
		{"recent + changed → working", now - 1, now - 5, "a", "b", now, true},
		{"recent + unchanged, last change just now → working (sticky)", now - 1, now - 2, "a", "a", now - 2, true},
		{"recent + unchanged, last change long ago → idle", now - 1, now - 10, "a", "a", now - 10, false},
		{"recent + unchanged, never changed → idle", now - 1, 0, "a", "a", 0, false},
		{"recent + changed even with old prevChange → working", now - 2, now - 100, "x", "y", now, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotChange, gotWorking := frameWorking(now, c.prevPoll, c.prevChange, c.prevHash, c.curHash)
			if gotChange != c.wantChange || gotWorking != c.wantWorking {
				t.Errorf("frameWorking = (change %d, working %v), want (%d, %v)",
					gotChange, gotWorking, c.wantChange, c.wantWorking)
			}
		})
	}
}

// TestPaneFrameWorking exercises the stateful wrapper end-to-end against a temp
// HOME: a changing pane becomes working on the second (recent) poll; a static
// pane stays idle.
func TestPaneFrameWorking(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// First poll: no baseline yet → idle, frame recorded.
	if PaneFrameWorking("%1", "frame-A", 100) {
		t.Fatal("first observation must report idle (no baseline)")
	}
	// Second poll 1s later, content changed → working.
	if !PaneFrameWorking("%1", "frame-B", 101) {
		t.Error("changed content within the poll window must report working")
	}
	// Third poll, content static and the last change has aged out → idle.
	if PaneFrameWorking("%1", "frame-B", 101+frameActiveSec+1) {
		t.Error("static content past the active window must report idle")
	}

	// A separate, always-static pane never reports working.
	if PaneFrameWorking("%2", "still", 200) {
		t.Fatal("first observation idle")
	}
	if PaneFrameWorking("%2", "still", 201) {
		t.Error("unchanged content must stay idle")
	}
}
