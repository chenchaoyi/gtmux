package terminal

import (
	"strings"
	"testing"
)

// iterm2 must satisfy Terminal and be reachable via the registry/detection.
var _ Terminal = iterm2{}

func TestITerm2Registered(t *testing.T) {
	t.Setenv("GTMUX_TERMINAL", "iterm2")
	if got := Active().Name(); got != "iTerm2" {
		t.Errorf("Active().Name() with GTMUX_TERMINAL=iterm2 = %q, want iTerm2", got)
	}
}

// SpawnTabs(dryRun) is the part testable without iTerm2 running: the generated
// AppleScript must target iTerm2 and attach each session.
func TestITerm2SpawnTabsScript(t *testing.T) {
	script, err := iterm2{}.SpawnTabs([]string{"work", "my proj"}, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`tell application "iTerm2"`, "attach -t 'work'", "attach -t 'my proj'", "create tab with default profile"} {
		if !strings.Contains(script, want) {
			t.Errorf("SpawnTabs script missing %q\n---\n%s", want, script)
		}
	}
}
