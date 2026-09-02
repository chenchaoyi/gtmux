package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// readHookEvents returns the events a settings file carries a gtmux entry for, plus
// whether the other tool's hook survived.
func readHookEvents(t *testing.T, path string) (map[string]bool, bool) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	foreign := false
	for ev, v := range asObject(m["hooks"]) {
		for _, raw := range asArray(v) {
			grp, _ := raw.(map[string]any)
			for _, h := range asArray(grp["hooks"]) {
				hm, _ := h.(map[string]any)
				if isGtmuxHookCommand(asString(hm["command"])) {
					have[ev] = true
				} else if asString(hm["command"]) != "" {
					foreign = true
				}
			}
		}
	}
	return have, foreign
}

// An incomplete install is fixable by exactly the same act as a missing one — doctor was
// sending it to the user as manual work because the step returned early on "installed".
// Measured on a real machine: PreCompact and PostCompact absent for weeks, the row
// telling the user to go run a command that --fix already knows how to run.
func TestFixTopsUpAnIncompleteClaudeHook(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := claudeSettingsPath()

	// Everything except the compaction pair, plus ANOTHER tool's hook on one of them —
	// the exact shape found in the wild.
	hooks := map[string]any{
		"PreCompact": []any{map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": "/other/tool/peon.sh"}},
		}},
	}
	for _, h := range hookEvents {
		if h.event == "PreCompact" || h.event == "PostCompact" {
			continue
		}
		hooks[h.event] = []any{map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": "/usr/local/bin/gtmux hook"}},
		}}
	}
	b, _ := json.Marshal(map[string]any{"hooks": hooks})
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	s := &fixState{yes: true, bin: "/usr/local/bin/gtmux"}
	if got := s.stepClaudeHook(); got != 1 {
		t.Fatalf("stepClaudeHook = %d, want 1 — an incomplete install is not manual work", got)
	}
	have, foreign := readHookEvents(t, path)
	for _, h := range hookEvents {
		if !have[h.event] {
			t.Errorf("%s still missing after the fix", h.event)
		}
	}
	if !foreign {
		t.Error("the other tool's hook was dropped — the fix must only touch gtmux's own entries")
	}
	if m := missingClaudeHookEvents(); m != nil {
		t.Errorf("doctor still reports %v missing after --fix ran", m)
	}
}

// A complete file is left alone: the step must not rewrite (and back up) a file that has
// nothing wrong with it, or every --fix run would churn the user's settings.
func TestFixLeavesACompleteClaudeHookAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	hooks := map[string]any{}
	for _, h := range hookEvents {
		hooks[h.event] = []any{map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": "/usr/local/bin/gtmux hook"}},
		}}
	}
	b, _ := json.Marshal(map[string]any{"hooks": hooks})
	if err := os.WriteFile(claudeSettingsPath(), b, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(claudeSettingsPath())
	if err != nil {
		t.Fatal(err)
	}

	s := &fixState{yes: true, bin: "/usr/local/bin/gtmux"}
	if got := s.stepClaudeHook(); got != 0 {
		t.Errorf("stepClaudeHook = %d on a complete file, want 0", got)
	}
	after, _ := os.ReadFile(claudeSettingsPath())
	if string(before) != string(after) {
		t.Error("a complete file was rewritten anyway")
	}
}
