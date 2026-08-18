package app

import (
	"strings"
	"testing"
	"time"
)

// The backstop exists so a stopped autosave can't quietly cost you your working set.
// It used to ask the wrong question — "is the autosave trigger written into
// status-right?" — and that question said YES on a machine where nothing had saved for
// six hours, because the trigger only runs while a terminal is attached and redrawing.
// The tests below pin the corrected question (has the FILE gone stale?) and the race
// lesson that survives it (a save that IS being written stands the backstop down).

// The save script must be invoked with "quiet". Without it resurrect forks a spinner that
// writes "Saving..." into tmux's message line on every client — the unexplained flicker —
// and displays a completion message on top.
func TestBackstopSaveRunsTheScriptQuietly(t *testing.T) {
	args := resurrectSaveArgs("/path/to/save.sh")
	if len(args) < 2 || args[len(args)-1] != "quiet" {
		t.Fatalf("save invocation = %v; want it to end with \"quiet\" (else it paints \"Saving...\" on every client)", args)
	}
	if !strings.HasSuffix(args[len(args)-2], "save.sh") {
		t.Errorf("save invocation = %v; want the script then \"quiet\"", args)
	}
}

const armedStatusRight = "#[fg=blue]%H:%M #(/Users/x/.tmux/plugins/tmux-continuum/scripts/continuum_save.sh)"

// THE BUG, in its own terms. The trigger is present — status-right says an autosave is
// armed — and yet nothing has written the save for hours, because the Mac was asleep and
// a status-bar trigger cannot fire without a status bar to draw. Under the old rule this
// read as "continuum is handling it" and the backstop never fired; 3.5 days of the
// commander's machine produced 76 such gaps, the longest just under six hours.
func TestBackstopFiresWhenAnArmedAutosaveIsNotActuallySaving(t *testing.T) {
	now := time.Now()
	dead := writeSaveWithAge(t, 6*time.Hour) // the measured worst case

	if !shouldBackstopSave(armedStatusRight, dead, now) {
		t.Error("a 6h-old save must trigger the backstop even with the trigger in status-right — " +
			"written into the status bar is not the same as running")
	}
	// A doubled trigger is the same story: still only a claim.
	if !shouldBackstopSave(armedStatusRight+" "+armedStatusRight, dead, now) {
		t.Error("two triggers that saved nothing for 6h are still saving nothing")
	}
}

// The other half: an autosave that IS working keeps the file fresh, and freshness is what
// stands the backstop down. This is the race guard — two concurrent save_all runs over the
// same files produced paired saves and a truncated pane_contents.tar.gz — now expressed as
// evidence rather than configuration.
func TestBackstopStandsDownWhileSavesKeepLanding(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name        string
		statusRight string
		age         time.Duration
	}{
		{"armed and saving every few minutes", armedStatusRight, 4 * time.Minute},
		{"armed, inside the wider armed grace", armedStatusRight, 15 * time.Minute},
		{"no trigger at all, but something saved recently", "#[fg=blue]%H:%M", 3 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if shouldBackstopSave(tc.statusRight, writeSaveWithAge(t, tc.age), now) {
				t.Errorf("a %v-old save must not trigger a second saver", tc.age)
			}
		})
	}
}

// With no trigger at all, nothing else can be saving, so the backstop steps in sooner.
// The gap between the two thresholds is the whole benefit of the doubt an armed autosaver
// gets — and the reason it is finite is the test above it.
func TestAnArmedAutosaverGetsMoreRopeThanAMissingOne(t *testing.T) {
	if backstopStaleAfter(armedStatusRight) <= backstopStaleAfter("#[fg=blue]%H:%M") {
		t.Fatal("an armed autosaver must get a WIDER grace than one that isn't there at all")
	}
	now := time.Now()
	between := writeSaveWithAge(t, 12*time.Minute) // past the disarmed line, inside the armed one
	if !shouldBackstopSave("#[fg=blue]%H:%M", between, now) {
		t.Error("no trigger + 12m of silence: nothing else is saving, so gtmux must")
	}
	if shouldBackstopSave(armedStatusRight, between, now) {
		t.Error("trigger present + 12m of silence: still inside the armed grace, leave it alone")
	}
}

// A save that isn't there at all is the most stale thing possible — never a reason to
// skip saving.
func TestBackstopFiresWhenThereIsNoSaveAtAll(t *testing.T) {
	if !shouldBackstopSave(armedStatusRight, "", time.Now()) {
		t.Error("no save file → the backstop must fire, armed or not")
	}
}
