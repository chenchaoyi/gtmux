package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// A one-shot goal that cannot be staged is refused before anything is created or typed.
// Any staging error used to fall back to the inline form with its whitespace collapsed,
// so "first\nsecond" went out as one joined line and was submitted with Enter (%12,
// 2026-10-06). Here $HOME/.local is a plain file, so the goals directory cannot exist,
// and tmux is a recorder: it must never be called.
func TestOneshotStagingFailureLaunchesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".local"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(t.TempDir(), "tmux-calls")
	rec := filepath.Join(t.TempDir(), "tmux")
	if err := os.WriteFile(rec, []byte("#!/bin/sh\necho \"$@\" >>"+calls+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	prev := tmux.Bin
	tmux.Bin = rec
	t.Cleanup(func() { tmux.Bin = prev })
	old := i18n.Lang()
	i18n.SetLang("en")
	t.Cleanup(func() { i18n.SetLang(old) })

	if _, _, err := oneshotCommand("claude", "", "first audit line\nsecond audit line"); err == nil {
		t.Fatal("oneshotCommand staged nothing and still returned a command")
	}
	var rc int
	errOut := captureStderr(t, func() {
		_, _, _, _, _, _, rc = spawnTarget("%9", "", "", "first audit line\nsecond audit line", "claude", "", "", true, false, true, false)
	})
	if rc == 0 || !strings.Contains(errOut, "nothing was launched") {
		t.Fatalf("rc %d, stderr %q", rc, errOut)
	}
	if b, _ := os.ReadFile(calls); len(b) != 0 {
		t.Fatalf("tmux was called: %s", b)
	}
}

// A short, plain goal stays inline with its bytes as they are (it used to be
// whitespace-collapsed); one with a control character is staged instead.
func TestOneshotInlineGoalKeepsItsBytes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd, staged, err := oneshotCommand("claude", "", "fix  the it's test")
	if err != nil || staged != "" || !strings.HasSuffix(cmd, ` -- 'fix  the it'\''s test'`) {
		t.Fatalf("inline: cmd %q staged %q err %v", cmd, staged, err)
	}
	cmd, staged, err = oneshotCommand("claude", "", "tab\there")
	if err != nil || staged == "" || !strings.Contains(cmd, "--goal-file") {
		t.Fatalf("a tab is staged: cmd %q staged %q err %v", cmd, staged, err)
	}
	if b, _ := os.ReadFile(staged); string(b) != "tab\there" {
		t.Fatalf("staged bytes %q", b)
	}
}
