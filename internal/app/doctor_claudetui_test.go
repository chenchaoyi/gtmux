package app

import (
	"encoding/json"
	"errors"
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
	if b, err := os.ReadFile(claudeTUIBackupPath()); err != nil || !strings.Contains(string(b), "DISABLE_ERROR_REPORTING") {
		t.Fatalf("no backup of the original: %v", err)
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

// Claude 2.1.285 reads 1/true/yes/on and 0/false/no/off, in any case, and a FALSE
// CLAUDE_CODE_NO_FLICKER forces the classic renderer (review M1).
func TestReadClaudeTUIFactsEnvSpellings(t *testing.T) {
	stubClaudePanes(t, 0, 0)
	cases := []struct {
		env        map[string]any
		osEnv      map[string]string
		forced, by string
	}{
		{map[string]any{"CLAUDE_CODE_NO_FLICKER": "yes"}, nil, "fullscreen", "CLAUDE_CODE_NO_FLICKER"},
		{map[string]any{"CLAUDE_CODE_NO_FLICKER": " ON "}, nil, "fullscreen", "CLAUDE_CODE_NO_FLICKER"},
		{map[string]any{"CLAUDE_CODE_NO_FLICKER": "off"}, nil, "default", "CLAUDE_CODE_NO_FLICKER"},
		{map[string]any{"CLAUDE_CODE_NO_FLICKER": "0"}, nil, "default", "CLAUDE_CODE_NO_FLICKER"},
		{map[string]any{"CLAUDE_CODE_NO_FLICKER": "maybe"}, nil, "", ""},
		{map[string]any{"CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN": "TRUE", "CLAUDE_CODE_NO_FLICKER": "1"}, nil, "default", "CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN"},
		// A false DISABLE_ALTERNATE_SCREEN forces nothing; NO_FLICKER still decides.
		{map[string]any{"CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN": "no", "CLAUDE_CODE_NO_FLICKER": "on"}, nil, "fullscreen", "CLAUDE_CODE_NO_FLICKER"},
		// Not in settings: the environment doctor runs in stands in for a launch shell.
		{map[string]any{}, map[string]string{"CLAUDE_CODE_NO_FLICKER": "yes"}, "fullscreen", "CLAUDE_CODE_NO_FLICKER"},
		// The settings env wins over the process env for the same variable.
		{map[string]any{"CLAUDE_CODE_NO_FLICKER": "false"}, map[string]string{"CLAUDE_CODE_NO_FLICKER": "1"}, "default", "CLAUDE_CODE_NO_FLICKER"},
	}
	for i, c := range cases {
		withClaudeSettings(t, map[string]any{"env": c.env})
		for k, v := range c.osEnv {
			t.Setenv(k, v)
		}
		f := readClaudeTUIFacts()
		if f.Forced != c.forced || f.ForcedBy != c.by {
			t.Errorf("case %d (%v / %v): forced %q by %q, want %q by %q", i, c.env, c.osEnv, f.Forced, f.ForcedBy, c.forced, c.by)
		}
	}
}

// An explicit env value — even an unusual spelling — is never overwritten by --fix (M1).
func TestFixLeavesAnExplicitEnvChoiceAlone(t *testing.T) {
	stubClaudePanes(t, 2, 2)
	path := withClaudeSettings(t, map[string]any{"env": map[string]any{"CLAUDE_CODE_NO_FLICKER": "yes"}})
	orig, _ := os.ReadFile(path)
	s := &fixState{yes: true}
	if got := s.stepClaudeTUI(); got != 0 {
		t.Fatalf("stepClaudeTUI = %d, want 0", got)
	}
	if now, _ := os.ReadFile(path); string(now) != string(orig) {
		t.Fatalf("file changed under an explicit env choice")
	}
}

// fullscreen in settings, classic forced by env: classic, not a warning --fix cannot clear (M2).
func TestClaudeTUIRowEnvOverridesASettingsFullscreen(t *testing.T) {
	f := claudeTUIFacts{Setting: "fullscreen", Forced: "default", ForcedBy: "CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN", Panes: 1}
	if got := claudeTUIRow(f); got.status != stOK || !strings.Contains(got.note, "overrides") {
		t.Fatalf("row = %+v", got)
	}
	f.AltPanes = 1
	if got := claudeTUIRow(f); got.status != stRec || !strings.Contains(got.value, "still fullscreen") {
		t.Fatalf("row with a fullscreen pane = %+v", got)
	}
	if claudeTUIPinnable(f) {
		t.Fatalf("pinnable under an env choice")
	}
}

// A settings file that cannot be read or parsed is said to be so; nothing is inferred
// from it and nothing is written (M3).
func TestClaudeTUIUnreadableSettings(t *testing.T) {
	stubClaudePanes(t, 1, 1)
	path := withClaudeSettings(t, map[string]any{"tui": "fullscreen"})
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := readClaudeTUIFacts()
	row := claudeTUIRow(f)
	if f.ReadErr == "" || row.status != stRec || !strings.Contains(row.value, "unreadable") || strings.Contains(row.note, "fresh install") {
		t.Fatalf("malformed: facts %+v row %+v", f, row)
	}
	s := &fixState{yes: true}
	if got := s.stepClaudeTUI(); got != 0 || s.rc != 0 {
		t.Fatalf("stepClaudeTUI on a malformed file = %d (rc %d), want 0 and nothing written", got, s.rc)
	}
	if b, _ := os.ReadFile(path); string(b) != "{not json" {
		t.Fatalf("malformed file was rewritten: %s", b)
	}
	if os.Getuid() == 0 {
		t.Skip("root reads a 0000 file")
	}
	if err := os.WriteFile(path, []byte(`{"tui":"fullscreen"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil { // WriteFile keeps an existing file's mode
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	stubClaudePanes(t, 0, 1)
	if row := claudeTUIRow(readClaudeTUIFacts()); row.status != stRec || !strings.Contains(row.value, "unreadable") {
		t.Fatalf("unreadable file reported %+v", row)
	}
}

func TestClaudeTUIRowUnknownValueAndNothingToCheck(t *testing.T) {
	// "classic" is not a value Claude knows: it counts as unset, and the note names it (L2).
	row := claudeTUIRow(claudeTUIFacts{Setting: "classic", AltPanes: 2, Panes: 2})
	if row.status != stRec || !strings.Contains(row.value, "fullscreen (2 of 2") || !strings.Contains(row.note, `"classic"`) {
		t.Fatalf("unknown value row = %+v", row)
	}
	if !claudeTUIPinnable(claudeTUIFacts{Setting: "classic", AltPanes: 1}) {
		t.Fatalf("an unknown value with a fullscreen pane should be fixable")
	}
	// No Claude pane to look at: not pinned, never "classic" (L1).
	row = claudeTUIRow(claudeTUIFacts{})
	if row.status != stInfo || strings.Contains(row.value, "classic") {
		t.Fatalf("nothing to check row = %+v", row)
	}
}

// A backup that cannot be written stops the change, returns failure and says nothing
// about pinning (L3).
func TestFixStopsWhenTheBackupCannotBeWritten(t *testing.T) {
	stubClaudePanes(t, 1, 1)
	path := withClaudeSettings(t, map[string]any{"theme": "auto"})
	orig, _ := os.ReadFile(path)
	if err := os.MkdirAll(claudeTUIBackupPath(), 0o755); err != nil { // a directory where the backup goes
		t.Fatal(err)
	}
	s := &fixState{yes: true}
	if got := s.stepClaudeTUI(); got != 0 || s.rc != 1 {
		t.Fatalf("stepClaudeTUI = %d, rc %d; want 0, rc 1", got, s.rc)
	}
	if now, _ := os.ReadFile(path); string(now) != string(orig) {
		t.Fatalf("settings changed without a backup")
	}
}

// A write that fails is a failure, not "pinned".
func TestFixReportsAFailedWrite(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root writes through 0500")
	}
	stubClaudePanes(t, 1, 1)
	path := withClaudeSettings(t, map[string]any{"theme": "auto"})
	// The backup can be written; the settings file itself cannot (it is written in place).
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	s := &fixState{yes: true}
	if got := s.stepClaudeTUI(); got != 0 || s.rc != 1 {
		t.Fatalf("stepClaudeTUI = %d, rc %d; want 0, rc 1", got, s.rc)
	}
}

// In one --fix run the hook step and this step both write settings.json; the file as it
// was before the run survives in the hook step's backup, and this step keeps its own (L3).
func TestHookAndRendererStepsKeepTheOriginalBackup(t *testing.T) {
	stubClaudePanes(t, 1, 1)
	path := withClaudeSettings(t, map[string]any{"theme": "auto"})
	orig, _ := os.ReadFile(path)
	if err := updateSettings(path, "/usr/local/bin/gtmux", true); err != nil { // the hook step
		t.Fatal(err)
	}
	afterHook, _ := os.ReadFile(path)
	s := &fixState{yes: true}
	if got := s.stepClaudeTUI(); got != 1 {
		t.Fatalf("stepClaudeTUI = %d, want 1", got)
	}
	if b, _ := os.ReadFile(path + ".gtmux.bak"); string(b) != string(orig) {
		t.Fatalf("the pre-run original was overwritten:\n%s", b)
	}
	if b, _ := os.ReadFile(claudeTUIBackupPath()); string(b) != string(afterHook) {
		t.Fatalf("renderer backup is not the file it changed")
	}
}

// Pinning keeps every other value as written: a number past 2^53 and "<>&" in a command (L4).
func TestPinKeepsBigNumbersAndLiteralCharacters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := claudeSettingsPath()
	raw := `{"n": 12345678901234567890, "hooks": {"Stop": [{"hooks": [{"type": "command", "command": "a <b> && c"}]}]}}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := pinClaudeTUIDefault(path); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	for _, want := range []string{"12345678901234567890", "a <b> && c", `"tui": "default"`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("%q lost:\n%s", want, b)
		}
	}
}

func TestCountClaudeAltPanes(t *testing.T) {
	out := "claude\t1\n2.1.285\t0\ncodex\t1\nbash\t0\nsleep\t1\n\nclaude\t1\n"
	if alt, total := countClaudeAltPanes(out); alt != 2 || total != 3 {
		t.Fatalf("alt %d total %d, want 2 of 3 (codex and sleep excluded)", alt, total)
	}
}

// A settings file that says `null` is valid JSON but not settings. It decoded into a nil map
// with no error, and the first write into that map crashed `doctor --fix` (review of #1284).
// Every reader now calls it unreadable, and nothing is written or backed up.
func TestNullSettingsAreUnreadableNotACrash(t *testing.T) {
	stubClaudePanes(t, 1, 1)
	path := withClaudeSettings(t, nil)
	if err := os.WriteFile(path, []byte("null\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := readClaudeTUIFacts()
	if row := claudeTUIRow(f); f.ReadErr == "" || !strings.Contains(row.value, "unreadable") {
		t.Fatalf("null settings: facts %+v row %+v", f, row)
	}
	s := &fixState{yes: true}
	if got := s.stepClaudeTUI(); got != 0 {
		t.Fatalf("stepClaudeTUI = %d, want 0", got)
	}
	if err := pinClaudeTUIDefault(path); !errors.Is(err, errNotJSONObject) {
		t.Fatalf("pinClaudeTUIDefault(null) = %v", err)
	}
	if err := updateSettings(path, "/usr/local/bin/gtmux", true); !errors.Is(err, errNotJSONObject) {
		t.Fatalf("updateSettings(null) = %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "null\n" {
		t.Fatalf("null settings rewritten: %q", b)
	}
	if _, err := os.Stat(claudeTUIBackupPath()); !os.IsNotExist(err) {
		t.Fatalf("a backup was written for a file that was never changed (%v)", err)
	}
}
