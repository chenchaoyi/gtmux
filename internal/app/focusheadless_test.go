package app

import (
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHeadlessFocusRequiresInteractiveClientAndChecksSelection(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	bin := filepath.Join(dir, "tmux")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$TEST_FOCUS_LOG"
case "$*" in
 *display-message*session_name*) echo work ;;
 *display-message*window_id*) echo @8 ;;
 *select-pane*) [ "$TEST_SELECT_FAIL" != 1 ] ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_FOCUS_LOG", log)
	old, interactive := tmux.Bin, headlessInteractive
	tmux.Bin = bin
	headlessInteractive = func() bool { return false }
	t.Cleanup(func() { tmux.Bin = old; headlessInteractive = interactive })
	if rc, _ := headlessFocus("%9"); rc != 2 {
		t.Fatalf("noninteractive focus = %d", rc)
	}
	b, _ := os.ReadFile(log)
	if strings.Contains(string(b), "select-") || strings.Contains(string(b), "switch-client") {
		t.Fatal("noninteractive focus changed an operator's tmux client")
	}
	headlessInteractive = func() bool { return true }
	if rc, _ := headlessFocus("%9"); rc != 0 {
		t.Fatalf("interactive focus = %d", rc)
	}
	t.Setenv("TEST_SELECT_FAIL", "1")
	if rc, _ := headlessFocus("%9"); rc == 0 {
		t.Fatal("failed pane selection reported success")
	}
}
