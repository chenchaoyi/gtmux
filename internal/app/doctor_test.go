package app

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDoctorCountersLine checks the tally + icon mapping per status level.
func TestDoctorCountersLine(t *testing.T) {
	n := &doctorCounters{}
	n.line(stOK, "a", "a")
	n.line(stOK, "b", "b")
	n.line(stRec, "c", "c")
	n.line(stMiss, "d", "d")
	n.line(stInfo, "e", "e") // info must not tally
	if n.ok != 2 || n.rec != 1 || n.miss != 1 {
		t.Fatalf("counts = ok %d rec %d miss %d, want 2/1/1", n.ok, n.rec, n.miss)
	}
}

// TestClaudeHookInstalled exercises the settings.json walk against a temp HOME:
// absent file, a non-gtmux hook, and a real gtmux hook command.
func TestClaudeHookInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if claudeHookInstalled() {
		t.Error("no settings.json → should report not installed")
	}

	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := claudeSettingsPath()

	if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":[{"hooks":[{"command":"/usr/bin/other thing"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if claudeHookInstalled() {
		t.Error("non-gtmux hook → should report not installed")
	}

	if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":[{"hooks":[{"command":"/opt/bin/gtmux hook"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !claudeHookInstalled() {
		t.Error("gtmux hook present → should report installed")
	}
}

// TestCodexNotifyIsGtmux: only a notify line referencing both gtmux and codex counts.
func TestCodexNotifyIsGtmux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if codexNotifyIsGtmux() {
		t.Error("no config.toml → not wired")
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, ".codex", "config.toml")

	if err := os.WriteFile(cfg, []byte(`notify = ["some-other-program"]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if codexNotifyIsGtmux() {
		t.Error("unrelated notify → not wired")
	}

	if err := os.WriteFile(cfg, []byte(`notify = ["/opt/bin/gtmux", "hook", "--agent", "codex"]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !codexNotifyIsGtmux() {
		t.Error("gtmux+codex notify → wired")
	}
}
