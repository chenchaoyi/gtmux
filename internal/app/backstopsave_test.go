package app

import (
	"strings"
	"testing"
)

// The backstop save exists for ONE case: continuum's trigger is missing from status-right,
// so nothing is autosaving and the save silently goes stale. When the trigger IS present,
// continuum is already saving — and a second saver is not redundancy, it is a race. Two
// concurrent resurrect save_all runs against the same files produced paired save files and
// a truncated pane_contents.tar.gz, i.e. gtmux corrupting the very save restore depends on.

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

// The gate: armed continuum → gtmux must not save at all.
func TestBackstopDefersToAnArmedContinuum(t *testing.T) {
	armed := "#[fg=blue]%H:%M #(/Users/x/.tmux/plugins/tmux-continuum/scripts/continuum_save.sh)"
	if shouldBackstopSave(armed) {
		t.Error("continuum is armed — a second saver races it and corrupts the archive")
	}
	if !shouldBackstopSave("#[fg=blue]%H:%M") {
		t.Error("no trigger — autosave is OFF, which is exactly what the backstop is for")
	}
	// A DOUBLED trigger is still "continuum is saving" — defer to it (and doctor reports
	// the duplication separately).
	if shouldBackstopSave(armed + " " + armed) {
		t.Error("continuum armed twice is still armed — must not add a third saver")
	}
}
