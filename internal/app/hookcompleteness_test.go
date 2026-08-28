package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeCodexHooks lays down a hooks file carrying gtmux entries for exactly the events
// named — the shape an older gtmux would have left behind.
func writeCodexHooks(t *testing.T, home string, events []string) {
	t.Helper()
	dir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	hooks := map[string]any{}
	for _, e := range events {
		hooks[e] = []any{map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": "/usr/local/bin/gtmux hook --agent codex"}},
		}}
	}
	b, _ := json.Marshal(map[string]any{"hooks": hooks})
	if err := os.WriteFile(filepath.Join(dir, "hooks.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// "Installed" and "installed completely" are different facts, and only the first was
// ever checked. Measured on a real machine: Codex carried 5 of the 9 events it should
// have, so tool and compaction events shipped days earlier were reaching nobody while
// doctor read ✓. Nothing rewrites an agent's hooks file on update — that is
// `install hooks`, and nobody re-runs it unprompted.
func TestMissingAgentHookEvents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")

	// The exact shape found in the wild: the pre-#855 set.
	writeCodexHooks(t, home, []string{"UserPromptSubmit", "PermissionRequest", "Stop", "SessionStart", "SessionEnd"})
	missing := missingAgentHookEvents("codex")
	if len(missing) == 0 {
		t.Fatal("a hooks file predating the current event set must be reported as incomplete")
	}
	want := map[string]bool{"PreToolUse": true, "PostToolUse": true, "PreCompact": true, "PostCompact": true}
	for _, m := range missing {
		if !want[m] {
			t.Errorf("reported %q as missing, which is not one of the events gtmux registers", m)
		}
		delete(want, m)
	}
	for m := range want {
		t.Errorf("%s is registered by gtmux but was not reported missing", m)
	}
}

// A complete file must be silent — a row that cries wolf on a healthy install is worse
// than no row.
func TestCompleteHooksReportNothingMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")

	var all []string
	for _, b := range agentInstallers["codex"].bindings {
		all = append(all, b.key)
	}
	writeCodexHooks(t, home, all)
	if missing := missingAgentHookEvents("codex"); len(missing) > 0 {
		t.Errorf("a complete install reported %v missing", missing)
	}
}

// No file is "not installed", which the caller already reports on its own; this check
// must not turn that into a different, noisier claim.
func TestNoHooksFileIsNotIncomplete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", "")
	if missing := missingAgentHookEvents("codex"); missing != nil {
		t.Errorf("with no hooks file at all, got %v", missing)
	}
}
