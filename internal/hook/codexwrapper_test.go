package hook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestCodexWrapperUsesVerifiedTerminalBeforeSessionRules(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	bin := filepath.Join(dir, "tmux")
	script := "#!/bin/sh\nprintf '%s\\n' '%7\tsample-launcher\t/example\twork:0.0\t100\t/dev/ttys001' '%8\tother-launcher\t/example\twork:0.1\t200\t/dev/ttys002' '%9\tcodex\t/other\tother:0.0\t300\t/dev/ttys003'\n"
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	oldBin, oldResolve := tmux.Bin, resolveWrappedCodex
	tmux.Bin = bin
	t.Cleanup(func() { tmux.Bin = oldBin; resolveWrappedCodex = oldResolve })
	resolveWrappedCodex = func(p []codexterminal.Pane) map[string]codexterminal.Client {
		if len(p) != 2 || p[0].Command != "sample-launcher" || p[0].PID != 100 || p[0].Cwd != "/example" || p[0].Loc != "work:0.0" || p[0].TTY != "/dev/ttys001" {
			t.Fatalf("incomplete wrapper candidates: %+v", p)
		}
		return map[string]codexterminal.Client{"%7": {PID: 101, Start: "birth"}}
	}
	panes := codexPanes()
	if len(panes) != 3 || panes[0].command != "codex" || panes[1].command != "other-launcher" || panes[2].command != "codex" {
		t.Fatalf("verified wrappers not passed to existing Codex rules: %+v", panes)
	}
	if got := codexPaneForCwd("%8", "/example", "s-7", panes, map[string]string{"%7": "s-7"}); got != "%7" {
		t.Fatalf("exact session did not choose verified wrapper: %s", got)
	}
	if got := codexWaitingPane("s-8", panes, map[string]string{"%8": "s-8"}); got != "" {
		t.Fatal("unverified launcher claimed permission hook")
	}
	resolveWrappedCodex = func([]codexterminal.Pane) map[string]codexterminal.Client { return nil }
	if got := codexPaneForCwd("%7", "/example", "s-7", codexPanes(), map[string]string{"%7": "s-7"}); got != "" {
		t.Fatal("display name/stale binding bypassed terminal verifier")
	}
}
