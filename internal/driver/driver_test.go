package driver

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRegistryMatchesFormerHookAgents pins the P0 zero-behavior guarantee: the
// registry's hook-equipped set is exactly the former dispatchbridge hookAgents
// whitelist — no agent gained or lost the event-first verify path in the move.
func TestRegistryMatchesFormerHookAgents(t *testing.T) {
	former := []string{
		"claude", "codex", "gemini", "cursor",
		"cursor-agent", "opencode", "copilot", "kiro",
	}
	for _, k := range former {
		if !For(k).HookEquipped {
			t.Errorf("For(%q).HookEquipped = false; the former whitelist had it", k)
		}
	}
	if len(registry) != len(former) {
		t.Errorf("registry has %d entries, former whitelist had %d", len(registry), len(former))
	}
}

func TestUnknownAgentIsLayerOne(t *testing.T) {
	d := For("mystery-agent")
	if d.HookEquipped || d.Receipt != nil {
		t.Errorf("unknown agent must resolve to the zero (Layer-1) driver, got %+v", d)
	}
	if d.Name != "mystery-agent" {
		t.Errorf("Name = %q, want the queried key", d.Name)
	}
}

// TestSwitchesDoNotAffectHookEquipped pins that the capability switches never
// touch the baseline hook-equipped fact (design §5): pre-driver behavior is not
// gated by driver config.
func TestSwitchesDoNotAffectHookEquipped(t *testing.T) {
	writeConfig(t, `{"driver": {"enable": false, "claude": {"receipt": false}}}`)
	if !For("claude").HookEquipped {
		t.Error("driver.enable=false must not strip the baseline HookEquipped fact")
	}
}

func TestSwitchParsing(t *testing.T) {
	cases := []struct {
		name     string
		cfg      string // "" = no config file
		enabled  bool
		claudeRc bool // capOn(claude, receipt)
	}{
		{"no config", "", true, true},
		{"empty driver", `{"driver": {}}`, true, true},
		{"global off", `{"driver": {"enable": false}}`, false, true},
		{"per-cap off", `{"driver": {"claude": {"receipt": false}}}`, true, false},
		{"explicit on", `{"driver": {"enable": true, "claude": {"receipt": true}}}`, true, true},
		{"malformed", `{not json`, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.cfg == "" {
				t.Setenv("HOME", t.TempDir())
			} else {
				writeConfig(t, c.cfg)
			}
			s := loadSwitches()
			if s.enabled() != c.enabled {
				t.Errorf("enabled() = %v, want %v", s.enabled(), c.enabled)
			}
			if s.capOn("claude", "receipt") != c.claudeRc {
				t.Errorf("capOn(claude, receipt) = %v, want %v", s.capOn("claude", "receipt"), c.claudeRc)
			}
		})
	}
}

// writeConfig points $HOME at a temp dir holding the given config.json.
func writeConfig(t *testing.T, content string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "gtmux")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
