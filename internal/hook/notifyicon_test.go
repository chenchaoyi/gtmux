package hook

import (
	"bytes"
	"os"
	"testing"

	"github.com/chenchaoyi/gtmux/assets"
)

// A notification for a built-in agent (Codex, …) must use THAT agent's icon, not the single
// cached Claude icon — the bug where a Codex session's macOS banner showed the CC logo.
func TestNotifyAgentIcon(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate state.Dir() from the real machine

	p := notifyAgentIcon("codex")
	if p == "" {
		t.Fatal("codex has a committed built-in icon; notifyAgentIcon must return a path")
	}
	got, err := os.ReadFile(p)
	if err != nil || len(got) == 0 {
		t.Fatalf("icon file not written: %v", err)
	}
	if !bytes.Equal(got, assets.AgentIcon("codex")) {
		t.Error("the cached notification icon must be the embedded codex icon, not Claude's")
	}
	// Claude has NO built-in icon (it uses its .app icon) → "" so the caller falls back to
	// the cached notify-icon.png (the Claude .app icon). That keeps Claude banners correct.
	if notifyAgentIcon("claude") != "" {
		t.Error("claude has no built-in icon; notifyAgentIcon must return empty (fall back)")
	}
}
