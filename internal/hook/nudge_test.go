package hook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNudgeLine(t *testing.T) {
	got := nudgeLine("permission", "gtmux:0.0", "%14", "⠙ fix the login bug")
	want := "[gtmux] waiting·permission gtmux:0.0 (%14) — ⠙ fix the login bug"
	if got != want {
		t.Errorf("nudgeLine = %q, want %q", got, want)
	}
	// A generic wait (no kind) and no title stay compact.
	if got := nudgeLine("", "", "%2", " "); got != "[gtmux] waiting (%2)" {
		t.Errorf("bare nudgeLine = %q", got)
	}
}

// hqNudgeEnabled: default ON (no file / no key), config false turns it off.
func TestHQNudgeEnabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if !hqNudgeEnabled() {
		t.Error("no config file should default to enabled")
	}
	dir := filepath.Join(os.Getenv("HOME"), ".config", "gtmux")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"autoResumeAgentSessions": false}`)
	if !hqNudgeEnabled() {
		t.Error("config without hqNudge key should default to enabled")
	}
	write(`{"hqNudge": false}`)
	if hqNudgeEnabled() {
		t.Error("hqNudge:false should disable")
	}
	write(`{"hqNudge": true}`)
	if !hqNudgeEnabled() {
		t.Error("hqNudge:true should enable")
	}
	write(`not json`)
	if !hqNudgeEnabled() {
		t.Error("unreadable config should default to enabled")
	}
}

// Without tmux (or with no hq pane), the nudge is a silent no-op — the hook must
// never fail an agent's turn over it.
func TestNudgeSupervisorNoop(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	nudgeSupervisor("%1", "permission") // must not panic, whatever the tmux state
	if pane := findSupervisorPane("%1"); pane != "" {
		t.Errorf("no hq session → findSupervisorPane = %q, want empty", pane)
	}
}
