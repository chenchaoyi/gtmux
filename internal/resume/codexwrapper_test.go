package resume

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/codexterminal"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestWrappedCodexBindingUsesVerifiedClientAndLauncher(t *testing.T) {
	target, _, _, now := bindingFixture(t, "codex-tui")
	dir := t.TempDir()
	bin := filepath.Join(dir, "tmux")
	t.Setenv("WRAPPER_TEST_CWD", target.Cwd)
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s\\t%s\\t%s\\t%s\\t%s\\t%s\\n' '%27' 'worker:0.0' \"$WRAPPER_TEST_CWD\" 58325 sample-launcher /dev/ttys001\n"), 0700); err != nil {
		t.Fatal(err)
	}
	oldBin, oldResolver := tmux.Bin, resolveWrappedCodexTarget
	tmux.Bin = bin
	t.Cleanup(func() { tmux.Bin = oldBin; resolveWrappedCodexTarget = oldResolver })
	resolveWrappedCodexTarget = func(p []codexterminal.Pane) map[string]codexterminal.Client {
		if len(p) != 1 || p[0].ID != "%27" || p[0].PID != 58325 || p[0].Cwd != target.Cwd || p[0].Loc != "worker:0.0" || p[0].Command != "sample-launcher" || p[0].TTY != "/dev/ttys001" {
			t.Fatalf("incomplete verifier input: %+v", p)
		}
		return map[string]codexterminal.Client{"%27": {PID: 58990, Start: "client-birth", RootStart: "root-birth", TTY: "ttys001"}}
	}
	live, ok := LiveCodexBindingTarget("%27")
	if !ok || live.PID != 58990 || live.PanePID != 58325 || live.ClientStart != "client-birth" || live.RootStart != "root-birth" || live.TTY != "ttys001" || live.Launcher != "sample-launcher" {
		t.Fatalf("wrapper identity lost: %+v, %v", live, ok)
	}
	wire, err := PrepareCodexBinding(live, "Repair the test issue", now)
	if err != nil {
		t.Fatal(err)
	}
	intent, ok := CodexBindingForPrompt(wire, now)
	if !ok {
		t.Fatal("fresh binding intent missing")
	}
	// The synthetic rollout is updated to the exact witness, as a submitted user
	// message would be. No real pane or model is used.
	dirLog := filepath.Join(os.Getenv("CODEX_HOME"), "sessions", "2026", "10", "03")
	files, err := filepath.Glob(filepath.Join(dirLog, "*.jsonl"))
	if err != nil || len(files) != 1 {
		t.Fatal("fixture rollout missing", err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatal(err)
		}
		if item["type"] == "response_item" {
			item["payload"] = map[string]any{"type": "message", "role": "user", "content": []any{map[string]string{"type": "input_text", "text": wire}}}
		}
		encoded, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(encoded))
	}
	data = []byte(strings.Join(lines, "\n") + "\n")
	if err := os.WriteFile(files[0], data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := CompleteCodexBinding(intent, live, "worker-session", now); err != nil {
		t.Fatal(err)
	}
	rec, ok := Load(live.Loc)
	if !ok || rec.Agent != "codex" || rec.Launcher != "sample-launcher" || rec.SessionID != "worker-session" {
		t.Fatalf("wrapper binding changed harness or lost launcher: %+v", rec)
	}
	resolveWrappedCodexTarget = func([]codexterminal.Pane) map[string]codexterminal.Client { return nil }
	if _, ok := LiveCodexBindingTarget("%27"); ok {
		t.Fatal("wrapper name authorized input without terminal proof")
	}
}

// This optional acceptance only observes metadata on an explicitly selected pane.
// It never prepares a witness, types a prompt or writes a binding.
func TestWrappedCodexLiveMetadata(t *testing.T) {
	pane := os.Getenv("GTMUX_TEST_WRAPPED_CODEX_PANE")
	if pane == "" {
		t.Skip("set GTMUX_TEST_WRAPPED_CODEX_PANE for read-only live terminal verification")
	}
	target, ok := LiveCodexBindingTarget(pane)
	if !ok || target.ClientStart == "" || target.RootStart == "" || target.TTY == "" {
		t.Fatal("selected wrapper lacks current terminal proof")
	}
	t.Logf("Verified current Codex terminal: pane=%s launcher=%s; no input or binding performed", pane, target.Launcher)
}
