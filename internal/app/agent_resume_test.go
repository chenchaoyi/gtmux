package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEffectiveResumeMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no config → default on

	// Flag overrides config.
	cases := []struct {
		flag string
		want resumeMode
	}{
		{"auto", resumeAuto},
		{"type", resumeType},
		{"off", resumeOff},
		{"", resumeAuto}, // no flag, no config → default auto
	}
	for _, c := range cases {
		restoreResumeFlag = c.flag
		if got := effectiveResumeMode(); got != c.want {
			t.Errorf("flag=%q → %v, want %v", c.flag, got, c.want)
		}
	}
	restoreResumeFlag = ""
}

func TestAutoResumeConfigToggle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := filepath.Join(home, ".config", "gtmux")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	restoreResumeFlag = "" // follow config

	// Explicitly off → type (pre-fill, don't run) per the agreed UX.
	if err := os.WriteFile(filepath.Join(cfg, "config.json"),
		[]byte(`{"autoResumeAgentSessions": false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := effectiveResumeMode(); got != resumeType {
		t.Errorf("config off → %v, want resumeType", got)
	}

	// Explicitly on → auto.
	os.WriteFile(filepath.Join(cfg, "config.json"), []byte(`{"autoResumeAgentSessions": true}`), 0o644)
	if got := effectiveResumeMode(); got != resumeAuto {
		t.Errorf("config on → %v, want resumeAuto", got)
	}
}

func TestIsShellCommand(t *testing.T) {
	for _, s := range []string{"bash", "zsh", "-zsh", "-bash", "fish", "sh"} {
		if !isShellCommand(s) {
			t.Errorf("isShellCommand(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"node", "claude", "codex", "python", "vim", ""} {
		if isShellCommand(s) {
			t.Errorf("isShellCommand(%q) = true, want false", s)
		}
	}
}
