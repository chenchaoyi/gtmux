package state

import (
	"os"
	"path/filepath"
	"testing"
)

// write creates a state file at <root>/<rel> with a marker payload.
func write(t *testing.T, root, rel string) string {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// The whole point: a pane id is a REUSED number, so a record whose pane is gone doesn't
// go quiet — after the next server restart it starts describing whoever inherited the
// number. This pins that every pane-keyed family is reconciled, not just the three
// turn-state dirs that were.
func TestReapDeadPaneStateClearsEveryPaneKeyedFamily(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := Dir()

	dead := []string{
		"active/%25", "waiting/%25", "finished/%25", "watched/%25",
		"enrolled/%25", "goal/%25", "goalchanged/%25", "awaited/%25",
		"bg/%25", "usagewarn/%25", "frame/%25", "cpu/%25", "watchdog/%25",
		"sends/%25.json",
		"hqwake/done-last-%25", "hqwake/resolved-claim-%25", "hqwake/resolved-track-%25",
	}
	var deadPaths []string
	for _, rel := range dead {
		deadPaths = append(deadPaths, write(t, root, rel))
	}
	// The same families for a pane that is still alive — none of these may be touched.
	live := []string{"enrolled/%7", "goal/%7", "sends/%7.json", "hqwake/done-last-%7"}
	var livePaths []string
	for _, rel := range live {
		livePaths = append(livePaths, write(t, root, rel))
	}

	n := ReapDeadPaneState(map[string]bool{"%7": true})

	if n != len(deadPaths) {
		t.Errorf("removed %d files, want %d — a family left out of the sweep is a family that keeps cross-wiring", n, len(deadPaths))
	}
	for i, p := range deadPaths {
		if exists(p) {
			t.Errorf("%s survived — its pane is gone", dead[i])
		}
	}
	for i, p := range livePaths {
		if !exists(p) {
			t.Errorf("%s was deleted — its pane is still live", live[i])
		}
	}
}

// The blast radius. These directories sit right beside the pane-keyed ones and a sweep
// that took them out would be far worse than the cruft it removes: `resume` is the only
// memory of which conversation belongs to which locator (and restore reads it at the one
// moment EVERY pane is gone), `usage` and `native` are keyed by conversation id, and
// hqwake's bookkeeping files carry HQ's consumption watermark.
func TestReapDeadPaneStateLeavesNonPaneKeyedStateAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := Dir()

	keep := []string{
		"resume/bXktc2Vzc2lvbjowLjA.json", // locator-keyed — restore depends on it
		"usage/c2Vzc18wMzk5Mjg0.json",     // conversation-keyed
		"native/sess-abc",                 // conversation-keyed
		"hqwake/consumed-seq",             // HQ's consumption watermark
		"hqwake/unread-state",
		"hqwake/selfrotate-state",
		"hqwake/last-seen-hq",
		"tasks/t123.json", // the dispatch ledger — `gtmux reap` needs it
		"last-finished",
	}
	var paths []string
	for _, rel := range keep {
		paths = append(paths, write(t, root, rel))
	}

	ReapDeadPaneState(map[string]bool{"%7": true})

	for i, p := range paths {
		if !exists(p) {
			t.Errorf("%s was deleted — it is not keyed by pane id", keep[i])
		}
	}
}

// An empty live set means "I could not ask tmux", never "there are no panes". Reading it
// the other way would reap the entire fleet's state on one failed query — the single
// worst thing this function could do.
func TestReapDeadPaneStateRefusesAnEmptyLiveSet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := write(t, Dir(), "enrolled/%3")

	if n := ReapDeadPaneState(nil); n != 0 {
		t.Errorf("removed %d files from an unknown live set; want 0", n)
	}
	if !exists(p) {
		t.Fatal("an unreadable live set must never reap anything")
	}
}

func TestIsPaneID(t *testing.T) {
	for _, ok := range []string{"%1", "%25", "%1024"} {
		if !IsPaneID(ok) {
			t.Errorf("IsPaneID(%q) = false, want true", ok)
		}
	}
	for _, no := range []string{"", "%", "25", "%2a", "consumed-seq", "unread-state", "%1.json", "done-last-%1"} {
		if IsPaneID(no) {
			t.Errorf("IsPaneID(%q) = true, want false", no)
		}
	}
}
