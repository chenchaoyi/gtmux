package hq

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agents"
)

func TestSID001ActualHQRecipeRetainsManagedFrontend(t *testing.T) {
	// Parse only the fixed product-generated recipe; do not start an agent.
	recipe := hqLaunchOptions("codex")
	b, err := exec.Command("/bin/sh", "-c", "set -- "+recipe+"; printf '%s\\000' \"$@\"").Output()
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Split(strings.TrimSuffix(string(b), "\x00"), "\x00")
	if len(argv) < 2 || argv[0] != "codex" || !agents.CodexBridgeEligible(argv[1:]) {
		t.Fatal("HQ generated an unmanaged launch", argv)
	}
	if !strings.Contains(recipe, "--approve-for-me") || !strings.Contains(recipe, "tui.notifications=") {
		t.Fatal("launch settings were removed instead of made compatible")
	}
}
