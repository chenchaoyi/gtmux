package app

import (
	"os"
	"path/filepath"
	"testing"
)

// Where you were left — the active window and pane — is the dimension nobody reported and
// every restore silently dropped: resurrect places it with `tmux switch-client`, which
// does nothing when no client is attached, and gtmux restores headlessly.

// writeSave plants a resurrect-format save. Field layout mirrors restore.sh's own awk.
func writeSave(t *testing.T, lines ...string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "tmux_resurrect_test.txt")
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseActiveSpotsFindsTheMarkedWindowAndPane(t *testing.T) {
	save := writeSave(t,
		"window\talpha\t0\t:one\t1\t:\tlayout0\t:",
		"window\talpha\t1\t:two\t1\t:*\tlayout1\t:", // '*' marks the session's active window
		"pane\talpha\t1\t:two\t:\t0\t:\t/tmp\t0\tzsh\t:",
		"pane\talpha\t1\t:two\t:\t2\t:\t/tmp\t1\tzsh\t:", // field 9 == 1 → the active pane
		"window\tbeta\t3\t:solo\t1\t:*\tlayoutB\t:",
	)
	got := parseActiveSpots(save)
	byName := map[string]activeSpot{}
	for _, s := range got {
		byName[s.session] = s
	}
	if a := byName["alpha"]; a.window != "1" || a.pane != "2" {
		t.Errorf("alpha = window %q pane %q; want window 1 pane 2", a.window, a.pane)
	}
	// A session whose save recorded no active pane still gets its window placed.
	if b := byName["beta"]; b.window != "3" || b.pane != "" {
		t.Errorf("beta = window %q pane %q; want window 3 and no pane", b.window, b.pane)
	}
}

// Window 0 is the default landing spot, so "no active marker" must NOT be reported as
// "window 0" — that would look identical to the bug being fixed.
func TestParseActiveSpotsIgnoresSessionsWithNoActiveWindow(t *testing.T) {
	save := writeSave(t,
		"window\talpha\t0\t:one\t1\t:\tlayout0\t:",
		"window\talpha\t1\t:two\t1\t:\tlayout1\t:",
	)
	if got := parseActiveSpots(save); len(got) != 0 {
		t.Errorf("got %+v; want none — no window was marked active", got)
	}
}

func TestParseActiveSpotsToleratesAMissingOrJunkSave(t *testing.T) {
	if got := parseActiveSpots(filepath.Join(t.TempDir(), "nope.txt")); got != nil {
		t.Errorf("missing save = %+v; want nil", got)
	}
	// Short/!tab lines must be skipped, not panic on a field index.
	save := writeSave(t, "", "window", "window\talpha", "state\tsomething", "pane\tbeta\t0")
	if got := parseActiveSpots(save); len(got) != 0 {
		t.Errorf("junk save = %+v; want none", got)
	}
}

// The pane marker is field 9; a pane that merely MENTIONS 1 elsewhere isn't active.
func TestParseActiveSpotsReadsTheActiveFlagFromTheRightField(t *testing.T) {
	save := writeSave(t,
		"window\talpha\t1\t:two\t1\t:*\tlayout1\t:",
		"pane\talpha\t1\t:two\t:\t1\t:\t/tmp\t0\tzsh\t:", // pane index 1, NOT active
	)
	got := parseActiveSpots(save)
	if len(got) != 1 || got[0].pane != "" {
		t.Errorf("got %+v; want alpha with NO active pane (field 9 was 0)", got)
	}
}
