package app

import (
	"os"
	"strings"
	"testing"
)

// With no target and stdin from /dev/null, install and uninstall say what to
// name instead of printing a menu nobody can answer and then "cancelled".
func TestInstallWithNoTargetOffATerminalSaysWhatToName(t *testing.T) {
	t.Setenv("GTMUX_LANG", "en")
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	old := os.Stdin
	t.Cleanup(func() { os.Stdin = old })
	os.Stdin = null

	for _, install := range []bool{true, false} {
		var got, errOut string
		out := captureStdout(t, func() {
			errOut = captureStderr(t, func() { got = askTarget(install) })
		})
		if got != "" {
			t.Errorf("install=%v: target %q, want none", install, got)
		}
		if strings.Contains(out, "What should gtmux") || strings.Contains(out, "cancelled") {
			t.Errorf("install=%v: printed the menu off a terminal:\n%s", install, out)
		}
		if !strings.Contains(errOut, "say what to work on: hooks | app | all") {
			t.Errorf("install=%v: stderr %q does not say what to name", install, errOut)
		}
	}
}
