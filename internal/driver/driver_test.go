package driver

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRegistryMatchesFormerHookAgents pins the migration guarantee: the set of
// agents whose Receipt is registered (the event-first verify path) is exactly the
// former dispatchbridge hookAgents whitelist — no agent gained or lost the
// event-first verify path across P0 (HookEquipped fact) → P1 (Receipt capability).
func TestRegistryMatchesFormerHookAgents(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no user config: all-on defaults
	former := []string{
		"claude", "codex", "gemini", "cursor",
		"cursor-agent", "opencode", "copilot", "kiro",
	}
	for _, k := range former {
		if For(k).Receipt == nil {
			t.Errorf("For(%q).Receipt = nil; the former whitelist had it hook-equipped", k)
		}
	}
	if len(registry) != len(former) {
		t.Errorf("registry has %d entries, former whitelist had %d", len(registry), len(former))
	}
}

func TestUnknownAgentIsLayerOne(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	d := For("mystery-agent")
	if d.Receipt != nil {
		t.Errorf("unknown agent must resolve to the zero (Layer-1) driver, got %+v", d)
	}
	if d.Name != "mystery-agent" {
		t.Errorf("Name = %q, want the queried key", d.Name)
	}
}

// TestSwitchForcesLayerOne pins design §5: turning the receipt capability off —
// per-agent or via the global enable — strips Receipt, so delivery verification
// runs the pure Layer-1 screen path for that agent.
func TestSwitchForcesLayerOne(t *testing.T) {
	writeConfig(t, `{"driver": {"claude": {"receipt": false}}}`)
	if For("claude").Receipt != nil {
		t.Error("driver.claude.receipt=false must strip Receipt (pure Layer-1 path)")
	}
	if For("codex").Receipt == nil {
		t.Error("a per-agent switch must not touch other agents")
	}
	writeConfig(t, `{"driver": {"enable": false}}`)
	if For("claude").Receipt != nil || For("codex").Receipt != nil {
		t.Error("driver.enable=false must strip every capability")
	}
	// Capabilities switch independently: ready off leaves receipt on (and v.v.).
	writeConfig(t, `{"driver": {"claude": {"ready": false}}}`)
	d := For("claude")
	if d.Ready != nil {
		t.Error("driver.claude.ready=false must strip Ready (full screen gate applies)")
	}
	if d.Receipt == nil {
		t.Error("the ready switch must not touch Receipt")
	}
	writeConfig(t, `{"driver": {"enable": false}}`)
	if For("claude").Ready != nil {
		t.Error("driver.enable=false strips Ready too")
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

// Content is registered exactly where a transcript parser exists — a pure
// re-wiring of transcript.Load — and switches off independently.
func TestContentRegistration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, k := range []string{"claude", "codex"} {
		if For(k).Content == nil {
			t.Errorf("For(%q).Content = nil; a parser exists for it", k)
		}
	}
	for _, k := range []string{"gemini", "cursor", "kiro", "unknown"} {
		if For(k).Content != nil {
			t.Errorf("For(%q).Content must be nil (no parser)", k)
		}
	}
	writeConfig(t, `{"driver": {"claude": {"content": false}}}`)
	d := For("claude")
	if d.Content != nil {
		t.Error("driver.claude.content=false must strip Content")
	}
	if d.Receipt == nil || d.Ready == nil {
		t.Error("the content switch must not touch other capabilities")
	}
}
