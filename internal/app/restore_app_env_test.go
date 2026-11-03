package app

import (
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreAppInheritedTmuxDoesNotTriggerNestedTerminalGuard(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TMUX", filepath.Join(home, "private-socket")+",123,0")
	t.Setenv("TMUX_PANE", "%997")
	bin := filepath.Join(home, "tmux")
	log := filepath.Join(home, "calls")
	t.Setenv("FIXTURE_CALLS", log)
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$FIXTURE_CALLS"
case "$*" in
 *has-session*) exit 0;;
 *list-sessions*) printf '0 synthetic-detached\n';exit 0;;
 *) exit 0;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	original := tmux.Bin
	tmux.Bin = bin
	defer func() { tmux.Bin = original; restoreFromApp = false }()
	if got := cmdRestore([]string{"--dry-run"}); got != 1 {
		t.Fatalf("terminal nested guard removed: %d", got)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("terminal guard ran tmux")
	}
	if got := cmdRestore([]string{"--from-app", "--dry-run"}); got != 0 {
		t.Fatalf("GUI inherited environment rejected: %d", got)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"kill-", "new-session", "attach-session", "send-keys", "source-file"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("dry fixture mutated topology: %s", forbidden)
		}
	}
	if !strings.Contains(string(data), "list-sessions") {
		t.Fatal("GUI never reached pending session lookup")
	}
	if os.Getenv("TMUX") == "" {
		t.Fatal("socket environment discarded")
	}
}
