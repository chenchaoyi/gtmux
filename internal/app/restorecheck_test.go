package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A tmux layout string carries three things: a checksum, the geometry, and the pane
// NUMBERS. Only the middle one describes the arrangement, and the other two change on
// every restore — so a raw comparison calls every window "changed" and is worth nothing.
func TestNormalizeLayoutSurvivesRenumberedPanes(t *testing.T) {
	before := "37e9,189x48,0,0{94x48,0,0,1,94x48,95,0,2}"    // as saved, panes %1 %2
	after := "b81c,189x48,0,0{94x48,0,0,7,94x48,95,0,8}"     // same window, panes %7 %8
	stacked := "d3a0,189x48,0,0[189x24,0,0,7,189x23,0,25,8]" // genuinely rearranged

	if normalizeLayout(before) != normalizeLayout(after) {
		t.Errorf("the same arrangement must compare equal after a server restart renumbered its panes:\n  %q\n  %q",
			normalizeLayout(before), normalizeLayout(after))
	}
	if normalizeLayout(before) == normalizeLayout(stacked) {
		t.Error("side-by-side and stacked must NOT compare equal — that is the whole question being asked")
	}
	if got, want := normalizeLayout(before), "189x48,0,0{94x48,0,0,94x48,95,0}"; got != want {
		t.Errorf("normalizeLayout = %q, want %q", got, want)
	}
	// A single-pane window.
	if got, want := normalizeLayout("d100,189x48,0,0,3"), "189x48,0,0"; got != want {
		t.Errorf("single-pane normalizeLayout = %q, want %q", got, want)
	}
}

// writeSave builds a resurrect save fixture from raw tab-joined field lists.
func writeShapeSave(t *testing.T, lines ...[]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "last")
	var b strings.Builder
	for _, f := range lines {
		b.WriteString(strings.Join(f, "\t"))
		b.WriteByte('\n')
	}
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func shapeWin(session, idx, layout string) []string {
	return []string{"window", session, idx, ":" + session + " win", "1", ":*", layout, ":"}
}
func shapePane(session, idx, paneIdx string) []string {
	return []string{"pane", session, idx, "1", ":*", paneIdx, "title", ":/tmp", "0", "bash", ":"}
}

func TestSavedShapesReadsPaneCountsAndLayout(t *testing.T) {
	save := writeShapeSave(t,
		shapeWin("gtmux dev", "0", "37e9,189x48,0,0{94x48,0,0,1,94x48,95,0,2}"),
		shapePane("gtmux dev", "0", "0"),
		shapePane("gtmux dev", "0", "1"),
		shapeWin("MP", "1", "d100,189x48,0,0,3"),
		shapePane("MP", "1", "0"),
	)
	got := savedShapes(save)
	if len(got) != 2 {
		t.Fatalf("parsed %d windows, want 2: %+v", len(got), got)
	}
	if got[0].key() != "gtmux dev:0" || got[0].Panes != 2 {
		t.Errorf("first window = %+v; want key \"gtmux dev:0\" with 2 panes", got[0])
	}
	if got[0].Layout != "189x48,0,0{94x48,0,0,94x48,95,0}" {
		t.Errorf("layout = %q; want it normalized", got[0].Layout)
	}
	if got[1].key() != "MP:1" || got[1].Panes != 1 {
		t.Errorf("second window = %+v; want key \"MP:1\" with 1 pane", got[1])
	}
}

// THE FAILURE THIS EXISTS TO CATCH. A restored window came back with one pane MORE than
// the save recorded (observed twice: 2026-08-15 and 2026-08-18). tmux then refuses the
// `select-layout` — "have 3 panes but need 2" — and resurrect throws that error away with
// `>/dev/null 2>&1`, so the window silently keeps its default arrangement and nothing,
// anywhere, says a word.
func TestLayoutDriftCatchesTheExtraPaneThatBreaksSelectLayout(t *testing.T) {
	saved := []windowShape{{Session: "gtmux dev", Index: "0", Panes: 2, Layout: "189x48,0,0{94x48,0,0,94x48,95,0}"}}
	live := []windowShape{{Session: "gtmux dev", Index: "0", Panes: 3, Layout: "189x48,0,0[189x16,0,0,189x15,0,17,189x15,0,33]"}}

	drift := layoutDrift(saved, live)
	if len(drift) != 1 {
		t.Fatalf("drift = %v; want exactly one line about gtmux dev:0", drift)
	}
	if !strings.Contains(drift[0], "gtmux dev:0") {
		t.Errorf("drift line %q must name the window", drift[0])
	}
	if !strings.Contains(drift[0], "2") || !strings.Contains(drift[0], "3") {
		t.Errorf("drift line %q must name both pane counts — that number IS the diagnosis", drift[0])
	}
}

func TestLayoutDriftCatchesARearrangedWindow(t *testing.T) {
	saved := []windowShape{{Session: "MP", Index: "1", Panes: 2, Layout: "189x48,0,0{94x48,0,0,94x48,95,0}"}}
	live := []windowShape{{Session: "MP", Index: "1", Panes: 2, Layout: "189x48,0,0[189x24,0,0,189x23,0,25]"}}
	if drift := layoutDrift(saved, live); len(drift) != 1 || !strings.Contains(drift[0], "MP:1") {
		t.Fatalf("side-by-side coming back stacked must be reported; got %v", drift)
	}
}

func TestLayoutDriftIsSilentWhenTheRestoreWasFaithful(t *testing.T) {
	saved := []windowShape{
		{Session: "MP", Index: "0", Panes: 2, Layout: "189x48,0,0{94x48,0,0,94x48,95,0}"},
		{Session: "MP", Index: "1", Panes: 1, Layout: "189x48,0,0"},
	}
	live := append([]windowShape(nil), saved...)
	// A session the user created AFTER the save is normal — reporting it would bury the
	// real signal under noise on every single restore.
	live = append(live, windowShape{Session: "scratch", Index: "0", Panes: 1, Layout: "80x24,0,0"})

	if drift := layoutDrift(saved, live); len(drift) != 0 {
		t.Fatalf("a faithful restore must say nothing; got %v", drift)
	}
}

func TestLayoutDriftReportsAWindowThatNeverCameBack(t *testing.T) {
	saved := []windowShape{{Session: "Diting", Index: "2", Panes: 1, Layout: "189x48,0,0"}}
	drift := layoutDrift(saved, []windowShape{{Session: "Diting", Index: "0", Panes: 1, Layout: "189x48,0,0"}})
	if len(drift) != 1 || !strings.Contains(drift[0], "Diting:2") {
		t.Fatalf("a saved window with no live counterpart must be reported; got %v", drift)
	}
}

// The 24-hour staleness warning is far too coarse for the case that actually cost work:
// the save behind the 2026-08-18 reboot was 37 minutes old — healthy by every threshold,
// and missing everything done in those 37 minutes, including the layout change the user
// later reported as "restore broke my layout".
func TestSaveAgeNoteNamesTheMomentBeingRestored(t *testing.T) {
	save := writeSaveWithAge(t, 37*time.Minute)
	now := time.Now() // after the file exists, so the age is exactly what was asked for
	note := saveAgeNote(save, now)
	if note == "" {
		t.Fatal("a restore must always say which moment it is restoring")
	}
	if !strings.Contains(note, "37m") {
		t.Errorf("note %q must carry the age — 37 minutes of lost work is invisible without it", note)
	}
	if !strings.Contains(note, now.Add(-37*time.Minute).Format("15:04")) {
		t.Errorf("note %q must carry the save's clock time", note)
	}
	if saveAgeNote("", now) != "" {
		t.Error("no save → no note")
	}
}

func TestShortAge(t *testing.T) {
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "<1m"},
		{37 * time.Minute, "37m"},
		{5*time.Hour + 55*time.Minute, "5h"},
		{50 * time.Hour, "2d"},
	} {
		if got := shortAge(c.d); got != c.want {
			t.Errorf("shortAge(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
