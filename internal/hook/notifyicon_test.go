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

	for _, key := range assets.AgentIconKeys() {
		t.Run(key, func(t *testing.T) {
			p := notifyAgentIcon(key)
			if p == "" {
				t.Fatal("embedded agent icon must have a notification cache path")
			}
			got, err := os.ReadFile(p)
			if err != nil || len(got) == 0 {
				t.Fatalf("icon file not written: %v", err)
			}
			if !bytes.Equal(got, assets.AgentIcon(key)) {
				t.Error("notification cache must contain this agent's canonical icon")
			}
		})
	}
	if notifyAgentIcon("no-such-agent") != "" {
		t.Error("an unknown agent must retain the app-icon fallback")
	}
}
