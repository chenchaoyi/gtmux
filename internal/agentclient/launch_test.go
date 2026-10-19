package agentclient

import (
	"strings"
	"testing"
)

func TestManagedLaunchSkipsLandingWithoutChangingClientIdentity(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	cmd, ok := Command("dsh", "dst", "")
	if !ok || !strings.Contains(cmd, "'DSH_TUI_NO_LAUNCHPAD=1'") || !strings.Contains(cmd, "'dst' '--backend' 'dsh' '.'") {
		t.Fatal("fresh launch did not target a chat composer", cmd)
	}
	cmd, ok = Command("dsh", "dst", "synthetic")
	if !ok || !strings.Contains(cmd, "'--resume' 'synthetic'") {
		t.Fatal("resume lost target", cmd)
	}
	if _, ok := Command("dst", "dst", ""); ok {
		t.Fatal("client became its own harness")
	}
}

func TestSavedClientCannotRestoreAnotherBackendAsDSH(t *testing.T) {
	host := "node /profile/node_modules/@deepseek-harness-tui/dsh-tui/lib/types/dsh-adapter/host-entry.js"
	if !MatchesSavedClient("dst", host+" --backend dsh") {
		t.Fatal("DSH client liveness rejected")
	}
	for _, tail := range []string{" --backend codex", " --backend=claude"} {
		if MatchesSavedClient("dst", host+tail) {
			t.Fatal("other backend borrowed DSH restore identity")
		}
	}
}
