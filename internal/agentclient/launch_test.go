package agentclient

import (
	"strings"
	"testing"
)

func TestManagedLaunchSkipsLandingWithoutChangingClientIdentity(t *testing.T) {
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
