package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeTUIRow(t *testing.T) {
	cases := []struct {
		name   string
		f      claudeTUIFacts
		status int
		value  string // a substring of the English value
	}{
		{"unset, every pane classic", claudeTUIFacts{Panes: 2}, stOK, "classic (not pinned)"},
		{"pinned classic", claudeTUIFacts{Setting: "default", Panes: 1}, stOK, "classic"},
		{"forced classic by env", claudeTUIFacts{Forced: "default", ForcedBy: "CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN"}, stOK, "classic"},
		// The 2026-10-03 shape: a reinstall reset settings.json and Claude chose fullscreen.
		{"unset, Claude chose fullscreen", claudeTUIFacts{AltPanes: 3, Panes: 3}, stRec, "fullscreen (3 of 3 sessions)"},
		{"set to fullscreen", claudeTUIFacts{Setting: "fullscreen"}, stRec, "fullscreen"},
		{"forced fullscreen by env", claudeTUIFacts{Forced: "fullscreen", ForcedBy: "CLAUDE_CODE_NO_FLICKER", AltPanes: 1, Panes: 1}, stRec, "CLAUDE_CODE_NO_FLICKER"},
		{"pinned, but old sessions still fullscreen", claudeTUIFacts{Setting: "default", AltPanes: 2, Panes: 3}, stRec, "2 session(s) still fullscreen"},
	}
	for _, c := range cases {
		got := claudeTUIRow(c.f)
		if got.status != c.status || !strings.Contains(got.value, c.value) {
			t.Errorf("%s: got (%d, %q), want (%d, …%q…)", c.name, got.status, got.value, c.status, c.value)
		}
	}
}

func TestClaudeTUIPinnableOnlyWithEvidenceAndNoExplicitChoice(t *testing.T) {
	for _, c := range []struct {
		f    claudeTUIFacts
		want bool
	}{
		{claudeTUIFacts{AltPanes: 1, Panes: 1}, true},
		{claudeTUIFacts{Setting: "fullscreen"}, true},
		{claudeTUIFacts{Panes: 2}, false},                                 // nothing wrong: do not touch the file
		{claudeTUIFacts{Setting: "default", AltPanes: 1}, false},          // already pinned; a restart is the fix
		{claudeTUIFacts{Forced: "fullscreen", AltPanes: 1}, false},        // the user forced it by env
		{claudeTUIFacts{Forced: "default", Setting: "fullscreen"}, false}, // env wins; leave it
	} {
		if got := claudeTUIPinnable(c.f); got != c.want {
			t.Errorf("pinnable(%+v) = %v, want %v", c.f, got, c.want)
		}
	}
}

// withClaudeSettings writes a settings.json under a temp HOME and returns its path.
func withClaudeSettings(t *testing.T, body map[string]any) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CODE_NO_FLICKER", "")
	t.Setenv("CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN", "")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(body, "", "  ")
	path := claudeSettingsPath()
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func stubClaudePanes(t *testing.T, alt, total int) {
	t.Helper()
	prev := claudeAltScreenPanes
	claudeAltScreenPanes = func() (int, int) { return alt, total }
	t.Cleanup(func() { claudeAltScreenPanes = prev })
}

func TestReadClaudeTUIFactsFromSettingsAndEnv(t *testing.T) {
	stubClaudePanes(t, 1, 2)
	withClaudeSettings(t, map[string]any{"tui": "fullscreen", "env": map[string]any{"CLAUDE_CODE_NO_FLICKER": "1"}})
	f := readClaudeTUIFacts()
	if f.Setting != "fullscreen" || f.Forced != "fullscreen" || f.ForcedBy != "CLAUDE_CODE_NO_FLICKER" || f.AltPanes != 1 || f.Panes != 2 {
		t.Fatalf("facts = %+v", f)
	}
	// DISABLE_ALTERNATE_SCREEN wins over NO_FLICKER, as it does in Claude.
	withClaudeSettings(t, map[string]any{"env": map[string]any{"CLAUDE_CODE_NO_FLICKER": "1", "CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN": "true"}})
	if f := readClaudeTUIFacts(); f.Forced != "default" {
		t.Fatalf("forced = %q, want default", f.Forced)
	}
}

// The fix writes one key and keeps the rest of the file — hooks, env, everything — as it
// was, after a backup.
func TestFixPinsClassicRendererKeepingEverythingElse(t *testing.T) {
	stubClaudePanes(t, 3, 3)
	before := map[string]any{
		"theme":         "auto",
		"env":           map[string]any{"DISABLE_ERROR_REPORTING": "1"},
		"hooks":         map[string]any{"Stop": []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "/other/tool.sh"}}}}},
		"modelSettings": map[string]any{"claude-opus-5-5": map[string]any{"effortLevel": "xhigh"}},
	}
	path := withClaudeSettings(t, before)
	s := &fixState{yes: true}
	if got := s.stepClaudeTUI(); got != 1 {
		t.Fatalf("stepClaudeTUI = %d, want 1", got)
	}
	after := readJSON(t, path)
	if after["tui"] != "default" {
		t.Fatalf("tui = %v, want default", after["tui"])
	}
	delete(after, "tui")
	wantB, _ := json.Marshal(before)
	gotB, _ := json.Marshal(after)
	if string(wantB) != string(gotB) {
		t.Fatalf("other keys changed:\nwant %s\ngot  %s", wantB, gotB)
	}
	if _, err := os.Stat(path + ".gtmux.bak"); err != nil {
		t.Fatalf("no backup: %v", err)
	}
	// Run again: already pinned, nothing more to do.
	if got := s.stepClaudeTUI(); got != 0 {
		t.Fatalf("second stepClaudeTUI = %d, want 0", got)
	}
}

func TestFixLeavesTheFileAloneWithoutEvidenceOrAgainstAnExplicitChoice(t *testing.T) {
	for _, c := range []struct {
		name  string
		body  map[string]any
		alt   int
		total int
	}{
		{"nothing wrong", map[string]any{"theme": "auto"}, 0, 2},
		{"forced fullscreen by env", map[string]any{"env": map[string]any{"CLAUDE_CODE_NO_FLICKER": "1"}}, 2, 2},
		{"already pinned", map[string]any{"tui": "default"}, 1, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			stubClaudePanes(t, c.alt, c.total)
			path := withClaudeSettings(t, c.body)
			orig, _ := os.ReadFile(path)
			s := &fixState{yes: true}
			if got := s.stepClaudeTUI(); got != 0 {
				t.Fatalf("stepClaudeTUI = %d, want 0", got)
			}
			now, _ := os.ReadFile(path)
			if string(now) != string(orig) {
				t.Fatalf("file changed:\n%s", now)
			}
		})
	}
}

func TestIsClaudeCommand(t *testing.T) {
	for cmd, want := range map[string]bool{"claude": true, "2.1.285": true, "codex": false, "bash": false, "": false} {
		if got := isClaudeCommand(cmd); got != want {
			t.Errorf("isClaudeCommand(%q) = %v, want %v", cmd, got, want)
		}
	}
}
