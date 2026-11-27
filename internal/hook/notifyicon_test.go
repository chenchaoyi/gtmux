package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/assets"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// A notification for a built-in agent (Codex, …) must use THAT agent's icon, not the single
// cached Claude icon — the bug where a Codex session's macOS banner showed the CC logo.
func TestNotifyAgentIcon(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate state.Dir() from the real machine

	for _, key := range assets.AgentIconKeys() {
		t.Run(key, func(t *testing.T) {
			p := radar.BuiltinIconPath(key)
			if p == "" {
				t.Fatal("embedded agent icon must have a notification cache path")
			}
			if p != filepath.Join(state.CacheDir(), "agent-icons", key+".png") {
				t.Fatalf("unexpected icon path %q", p)
			}
			before, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			again, err := os.Stat(radar.BuiltinIconPath(key))
			if err != nil || !os.SameFile(before, again) {
				t.Fatal("cache hit replaced the icon")
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
	if radar.BuiltinIconPath("no-such-agent") != "" {
		t.Error("an unknown agent must omit the thumbnail")
	}
}
