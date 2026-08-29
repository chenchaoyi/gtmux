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

// writeClaudeSettings lays down a ~/.claude/settings.json carrying gtmux entries for
// exactly the events named, alongside another tool's hook that must survive untouched.
func writeClaudeSettings(t *testing.T, home string, events []string) {
	t.Helper()
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	hooks := map[string]any{
		// Someone else's hook on an event gtmux also wants: gtmux is absent from it, so
		// the event is MISSING for gtmux even though the key exists. This is the shape
		// that was found on the real machine, and a check keyed on the key alone would
		// call it present.
		"PreCompact": []any{map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": "/Users/x/.claude/hooks/peon-ping/peon.sh"}},
		}},
	}
	for _, e := range events {
		hooks[e] = []any{map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": "/usr/local/bin/gtmux hook"}},
		}}
	}
	b, _ := json.Marshal(map[string]any{"hooks": hooks})
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The same question for Claude, which the codex-shaped check could not see: Claude's
// hooks come from hookEvents, not from an agentInstaller. Found on a real machine on
// 2026-08-29 — PreCompact and PostCompact had been in gtmux's list for weeks, were in
// neither the settings file nor any of 28291 events over 48 days, and the row still read
// a plain green "installed" while compaction fired 61 times in four transcripts.
func TestMissingClaudeHookEvents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Everything except the two compaction events — the exact gap measured in the wild.
	var all []string
	for _, h := range hookEvents {
		if h.event != "PreCompact" && h.event != "PostCompact" {
			all = append(all, h.event)
		}
	}
	writeClaudeSettings(t, home, all)

	missing := missingClaudeHookEvents()
	if len(missing) != 2 || missing[0] != "PostCompact" || missing[1] != "PreCompact" {
		t.Fatalf("missing = %v, want exactly [PostCompact PreCompact] — another tool's hook on the\n"+
			"same event does not make it present for gtmux", missing)
	}

	// A complete file is silent.
	var every []string
	for _, h := range hookEvents {
		every = append(every, h.event)
	}
	writeClaudeSettings(t, home, every)
	if m := missingClaudeHookEvents(); m != nil {
		t.Errorf("a complete file reported %v", m)
	}

	// No gtmux entry at all is "not installed", which the row already says in its own
	// words — reporting every event as missing there would be a second, noisier claim.
	writeClaudeSettings(t, home, nil)
	if m := missingClaudeHookEvents(); m != nil {
		t.Errorf("an uninstalled file reported %v — that is the other row's fact", m)
	}
}
