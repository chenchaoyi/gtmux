package app

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func useDoctorTmuxStub(t *testing.T, script string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tmux")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	previous := tmux.Bin
	tmux.Bin = path
	t.Cleanup(func() { tmux.Bin = previous })
}

func TestDoctorUnreadableTmuxDoesNotRecommendConfigChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	useDoctorTmuxStub(t, "echo 'error connecting to test socket (Operation not permitted)' >&2\nexit 1\n")
	rows := tmuxSettingsChecks()
	if len(rows) != 1 || rows[0].status != stMiss || !strings.Contains(rows[0].value, "Operation not permitted") {
		t.Fatalf("want one actionable connection failure, got %+v", rows)
	}
	for _, row := range restoreRebootChecks() {
		if row.label == "capture-pane" || row.label == "auto-restore" {
			t.Fatalf("unreadable restore settings reported as configuration issues: %+v", row)
		}
	}
	conf := filepath.Join(home, ".tmux.conf")
	const original = "set -g mouse on\n"
	if err := os.WriteFile(conf, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		previous := os.Stderr
		os.Stderr = os.Stdout
		defer func() { os.Stderr = previous }()
		if rc := doctorFix(true, nil); rc != 1 {
			t.Errorf("unreadable tmux fix exit = %d, want 1", rc)
		}
	})
	if !strings.Contains(out, "Operation not permitted") || !strings.Contains(out, "socket") {
		t.Fatalf("preflight did not explain the error and recovery: %q", out)
	}
	contents, err := os.ReadFile(conf)
	if err != nil || string(contents) != original {
		t.Fatalf("failed preflight changed config: %q (%v)", contents, err)
	}
	if _, err := os.Stat(conf + ".gtmux.bak"); !os.IsNotExist(err) {
		t.Fatalf("failed preflight created a backup: %v", err)
	}
}

func TestApplyConfReportsPartialLiveFailure(t *testing.T) {
	// A failure on the first option must be reported, while subsequent options
	// are still attempted and the saved config remains available for recovery.
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	marker := filepath.Join(dir, "second-applied")
	t.Setenv("DOCTOR_TEST_MARKER", marker)
	useDoctorTmuxStub(t, "case \"$4\" in\nset-titles) echo 'permission denied' >&2; exit 1;;\nhistory-limit) touch \"$DOCTOR_TEST_MARKER\";;\nesac\n")
	s := &fixState{confPath: filepath.Join(dir, ".tmux.conf")}
	var changed int
	out := captureStdout(t, func() {
		previous := os.Stderr
		os.Stderr = os.Stdout
		defer func() { os.Stderr = previous }()
		changed = s.applyConf([]string{"set -g set-titles on", "set -g history-limit 50000"},
			[][]string{{"set", "-g", "set-titles", "on"}, {"set", "-g", "history-limit", "50000"}})
	})
	if changed != 0 || s.rc != 1 || strings.Contains(out, "updated") || strings.Contains(out, "已更新") {
		t.Fatalf("live failure claimed success: changed=%d rc=%d output=%q", changed, s.rc, out)
	}
	if !strings.Contains(out, "permission denied") || !strings.Contains(out, "set-titles") || !strings.Contains(out, tildeify(s.confPath)) {
		t.Fatalf("missing failed command, underlying error, or saved path: %q", out)
	}
	contents, err := os.ReadFile(s.confPath)
	if err != nil || !strings.Contains(string(contents), "history-limit 50000") {
		t.Fatalf("persisted fix was lost: %q (%v)", contents, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("second option was not attempted: %v", err)
	}
}

func TestFixLiveSettingsPassRecheck(t *testing.T) {
	if tmux.Bin == "" {
		t.Skip("tmux unavailable")
	}
	dir, err := os.MkdirTemp("/tmp", "gtx-doctor-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_TMPDIR", dir)
	t.Setenv("HOME", dir)
	if _, err := tmux.Run("-f", "/dev/null", "new-session", "-d", "-s", "doctor-probe", "sleep 60"); err != nil {
		t.Fatalf("start isolated tmux: %v", err)
	}
	t.Cleanup(func() { _, _ = tmux.Run("kill-server") })
	if got, err := tmux.Run("list-sessions", "-F", "#{session_name}"); err != nil || got != "doctor-probe" {
		t.Fatalf("refusing to configure non-isolated tmux: %q (%v)", got, err)
	}
	s := &fixState{confPath: filepath.Join(dir, ".tmux.conf"), yes: true}
	for _, step := range []func() int{s.stepSetTitles, s.stepRestoreSettings} {
		if n := step(); n != 1 || s.rc != 0 {
			t.Fatalf("fix = %d rc=%d, want successful application", n, s.rc)
		}
	}
	for _, row := range []dcheck{rowSetTitles(), rowHistory(), rowCapture(), rowAutoRestore()} {
		if row.status != stOK {
			t.Errorf("applied option failed recheck: %+v", row)
		}
	}
	if n := s.stepSetTitles() + s.stepRestoreSettings(); n != 0 {
		t.Fatalf("healthy settings re-offered %d fixes", n)
	}
}

func TestFixSummaryIncludesUnresolvedConfigAndErrors(t *testing.T) {
	previous := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(previous) })
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		for _, applied := range []int{0, 1} {
			s := &fixState{}
			out := captureStdout(t, func() {
				if rc := s.finish(applied, []dsection{{"tmux", []dcheck{{stRec, "history-limit", "2000", "raise to ~50000"}}}}); rc != 0 {
					t.Errorf("recommendation should not block: %d", rc)
				}
			})
			if !strings.Contains(out, "2000") || !strings.Contains(out, "raise to ~50000") ||
				strings.Contains(out, "not a config change") || strings.Contains(out, "auto-fixable") || strings.Contains(out, "不是配置项") || strings.Contains(out, "不能自动修") {
				t.Errorf("misleading summary for %s, %d fixes: %q", lang, applied, out)
			}
		}
		for _, tc := range []struct {
			rc   int
			rows []dcheck
		}{
			{0, []dcheck{{stMiss, "set-titles", "not set", "required"}}},
			{1, nil}, // A later healthy read must not hide an earlier application error.
		} {
			s := &fixState{rc: tc.rc}
			out := captureStdout(t, func() {
				if rc := s.finish(1, []dsection{{"tmux", tc.rows}}); rc != 1 {
					t.Errorf("failure exit = %d, want 1", rc)
				}
			})
			if strings.Contains(out, "Done.") || strings.Contains(out, "都配好了") || strings.Contains(out, "完成，重新") {
				t.Errorf("failure summary claimed success: %q", out)
			}
		}
	}
}

// Exercise the actual fix against a separate tmux server. The important outcome is
// a single live save hook after config reloads while the user's status text remains.
func TestFixStepArmsResurrectAutoSave(t *testing.T) {
	if tmux.Bin == "" {
		t.Skip("tmux unavailable")
	}
	dir, err := os.MkdirTemp("/tmp", "gtmux-autosave-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("HOME", dir)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_TMPDIR", dir)
	plugin := filepath.Join(dir, ".tmux", "plugins", "tmux-continuum", "scripts")
	if err := os.MkdirAll(plugin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, "continuum_save.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := tmux.Run("-f", "/dev/null", "new-session", "-d", "-s", "autosave-probe", "sleep 30"); err != nil {
		t.Skipf("cannot start isolated tmux: %v", err)
	}
	t.Cleanup(func() { _, _ = tmux.Run("kill-server") })
	if got, err := tmux.Run("list-sessions", "-F", "#{session_name}"); err != nil || got != "autosave-probe" {
		t.Fatalf("tmux server was not isolated: %q (%v)", got, err)
	}
	if _, err := tmux.Run("set", "-g", "status-right", "clock %H:%M"); err != nil {
		t.Fatal(err)
	}
	s := &fixState{confPath: filepath.Join(dir, ".tmux.conf"), yes: true}
	if n := s.stepAutoSave(); n != 1 || s.rc != 0 {
		t.Fatalf("fix result = %d, rc = %d; want one successful change", n, s.rc)
	}
	assertStatus := func() {
		t.Helper()
		got, err := tmux.Run("show", "-gv", "status-right")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(got, "clock %H:%M") || continuumTriggerCount(got) != 1 || !strings.Contains(got, plugin) {
			t.Errorf("custom status was lost or autosave not armed once: %q", got)
		}
	}
	assertStatus()
	conf, err := os.ReadFile(s.confPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(conf), "if-shell -F") || !strings.Contains(string(conf), plugin) || strings.Contains(string(conf), "#(~") {
		t.Fatalf("config lacks guarded absolute hook: %s", conf)
	}
	if n := s.stepAutoSave(); n != 0 {
		t.Errorf("second fix made %d changes; want zero", n)
	}
	for i := 0; i < 2; i++ {
		if _, err := tmux.Run("source-file", s.confPath); err != nil {
			t.Fatal(err)
		}
		assertStatus()
	}
	// TPM can inject the absolute hook before the managed block is sourced.
	trigger := " #(" + filepath.Join(plugin, "continuum_save.sh") + ")"
	if _, err := tmux.Run("set", "-g", "status-right", "clock %H:%M"+trigger); err != nil {
		t.Fatal(err)
	}
	if _, err := tmux.Run("source-file", s.confPath); err != nil {
		t.Fatal(err)
	}
	assertStatus()
	// A doubled hook needs human review; adding a third would make every save
	// run three times. A missing script is likewise not a safe repair target.
	if _, err := tmux.Run("set", "-g", "status-right", "clock %H:%M"+trigger+trigger); err != nil {
		t.Fatal(err)
	}
	if n := s.stepAutoSave(); n != 0 {
		t.Errorf("duplicate trigger led to %d changes; want zero", n)
	}
	if _, err := tmux.Run("set", "-g", "status-right", "clock %H:%M"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(plugin, "continuum_save.sh")); err != nil {
		t.Fatal(err)
	}
	if n := s.stepAutoSave(); n != 0 {
		t.Errorf("missing save script led to %d changes; want zero", n)
	}
}

func TestManagedKey(t *testing.T) {
	cases := map[string]string{
		"set -g set-titles on":                         "set-titles",
		"set -g @continuum-restore 'on'":               "@continuum-restore",
		"set -g set-titles-string '#S — #W'":           "set-titles-string",
		"run '~/.tmux/plugins/tpm/tpm'":                "run",
		"set -g @plugin 'tmux-plugins/tmux-resurrect'": "@plugin",
	}
	for line, want := range cases {
		if got := managedKey(line); got != want {
			t.Errorf("managedKey(%q) = %q, want %q", line, got, want)
		}
	}
}

func TestUpsertManagedBlock(t *testing.T) {
	// Append into a non-empty config preserves the user's lines.
	out := upsertManagedBlock("set -g mouse on\n", []string{"set -g set-titles on"})
	if !strings.Contains(out, "set -g mouse on") {
		t.Error("user line was dropped")
	}
	if strings.Count(out, fixBlockBegin) != 1 || strings.Count(out, fixBlockEnd) != 1 {
		t.Errorf("want exactly one managed block, got:\n%s", out)
	}

	// Re-running replaces the block in place (idempotent — still ONE block).
	out2 := upsertManagedBlock(out, []string{"set -g set-titles on", "set -g history-limit 50000"})
	if strings.Count(out2, fixBlockBegin) != 1 {
		t.Errorf("upsert must not duplicate the block:\n%s", out2)
	}
	if !strings.Contains(out2, "history-limit 50000") || !strings.Contains(out2, "set -g mouse on") {
		t.Errorf("upsert lost content:\n%s", out2)
	}

	// Empty config → just the block.
	if e := upsertManagedBlock("", []string{"set -g set-titles on"}); !strings.HasPrefix(e, fixBlockBegin) {
		t.Errorf("empty conf should start with the block, got:\n%s", e)
	}
}

func TestMergeManagedLines(t *testing.T) {
	// Existing block has set-titles; a later run adds history-limit. Union, no dup.
	conf := upsertManagedBlock("", []string{"set -g set-titles on"})
	merged := mergeManagedLines(conf, []string{"set -g set-titles on", "set -g history-limit 50000"})
	if len(merged) != 2 {
		t.Fatalf("want 2 merged lines (deduped), got %d: %v", len(merged), merged)
	}

	// The TPM run line is always floated last.
	conf2 := upsertManagedBlock("", []string{"run '~/.tmux/plugins/tpm/tpm'", "set -g @plugin 'x'"})
	merged2 := mergeManagedLines(conf2, []string{"set -g history-limit 50000"})
	if managedKey(merged2[len(merged2)-1]) != "run" {
		t.Errorf("run line must be last, got: %v", merged2)
	}
}

// TestTpmWiringLines guards the TPM wiring: three @plugin declarations followed
// by the run line LAST (TPM must initialize after the plugins are declared).
func TestTpmWiringLines(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	lines := tpmWiringLines()
	if managedKey(lines[len(lines)-1]) != "run" {
		t.Errorf("run line must be last, got: %v", lines)
	}
	if c := strings.Count(strings.Join(lines, "\n"), "@plugin"); c != 3 {
		t.Errorf("want 3 @plugin declarations, got %d", c)
	}
}

// TestAnswerYes guards the confirm() safety fix: Enter / y / yes = yes, but EOF
// with no input (redirected stdin) must NOT auto-confirm a mutating action.
func TestAnswerYes(t *testing.T) {
	cases := []struct {
		line    string
		err     error
		want    bool
		comment string
	}{
		{"\n", nil, true, "Enter = default yes"},
		{"y\n", nil, true, "y"},
		{"yes\n", nil, true, "yes"},
		{"Y\n", nil, true, "uppercase Y"},
		{"n\n", nil, false, "n"},
		{"no\n", nil, false, "no"},
		{"", io.EOF, false, "EOF, no input → NOT yes"},
		{"y", io.EOF, true, "y then EOF (no newline) → yes"},
	}
	for _, c := range cases {
		if got := answerYes(c.line, c.err); got != c.want {
			t.Errorf("answerYes(%q, %v) = %v, want %v (%s)", c.line, c.err, got, c.want, c.comment)
		}
	}
}

func TestCodexHooksFeatureEnabled(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"model = \"x\"\n", false},
		{"features.hooks = true\n", true},
		{"features.hooks=true\n", true},
		{"features.hooks = false\n", false},
		{"[features]\nhooks = true\n", true},
		{"[features]\nother = 1\nhooks = true\n", true},
		// hooks=true under a DIFFERENT table doesn't count.
		{"[other]\nhooks = true\n", false},
		{"[features]\nhooks = false\n", false},
	}
	for _, c := range cases {
		if got := codexHooksFeatureEnabled(c.in); got != c.want {
			t.Errorf("codexHooksFeatureEnabled(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestEnableHooksUnderFeatures pins the bug the doctor hit: when a [features]
// table already exists, --fix must actually WRITE `hooks = true` under it (not
// just print guidance), so a follow-up `doctor` reports wired.
func TestEnableHooksUnderFeatures(t *testing.T) {
	cases := []struct {
		name, in string
	}{
		// The exact shape from the field report: a [features] table with an
		// unrelated key, followed by another table that must survive intact.
		{"insert under table", "model = \"x\"\n\n[features]\njs_repl = false\n\n[mcp_servers.node_repl]\ncmd = \"node\"\n"},
		// Flip an explicit hooks = false rather than duplicating the key.
		{"flip false", "[features]\nhooks = false\nother = 1\n"},
		// Insert when the [features] table is empty.
		{"empty table", "[features]\n"},
		// No table → dotted top-level fallback.
		{"no table", "model = \"x\"\n"},
	}
	for _, c := range cases {
		out := enableHooksUnderFeatures(c.in)
		if !codexHooksFeatureEnabled(out) {
			t.Errorf("%s: features.hooks not enabled after fix:\n%s", c.name, out)
		}
		// The foreign table + key must be preserved.
		if strings.Contains(c.in, "[mcp_servers.node_repl]") && !strings.Contains(out, "[mcp_servers.node_repl]") {
			t.Errorf("%s: dropped a foreign table:\n%s", c.name, out)
		}
		// A flip must not leave a stray `hooks = false` behind.
		if c.name == "flip false" && strings.Contains(out, "hooks = false") {
			t.Errorf("%s: left a stale hooks = false:\n%s", c.name, out)
		}
	}
}

func TestTomlHasTable(t *testing.T) {
	if !tomlHasTable("[features]\nx = 1\n", "features") {
		t.Error("should find [features]")
	}
	if tomlHasTable("features.hooks = true\n", "features") {
		t.Error("dotted key is not a [features] table")
	}
	if tomlHasTable("[features.sub]\n", "features") {
		t.Error("[features.sub] is a different table")
	}
}

func TestInsertTomlTopLevel(t *testing.T) {
	line := "notify = [\"g\"]"

	// empty file → just the line
	if got := insertTomlTopLevel("", line); got != line+"\n" {
		t.Errorf("empty: got %q", got)
	}
	// no tables → appended, key stays top-level
	got := insertTomlTopLevel("model = \"x\"\n", line)
	if !strings.Contains(got, "model = \"x\"") || !strings.Contains(got, line) {
		t.Errorf("no-table: got %q", got)
	}
	// with a table → inserted BEFORE the table header (stays top-level)
	got = insertTomlTopLevel("model=\"x\"\n[mcp.y]\nfoo=1\n", line)
	ni, ti := strings.Index(got, "notify"), strings.Index(got, "[mcp.y]")
	if ni < 0 || ti < 0 || ni > ti {
		t.Errorf("notify must come before the table header:\n%s", got)
	}
}
