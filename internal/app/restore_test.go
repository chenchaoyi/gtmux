package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// TestEnsureServerRestoresViaResurrect drives the real ensureServer on an
// ISOLATED tmux server (TMUX_TMPDIR) with a fake restore.sh standing in for
// tmux-resurrect, proving the trigger→wait→drop-boot glue. Gated (needs tmux);
// run with GTMUX_IT=1.
func TestEnsureServerRestoresViaResurrect(t *testing.T) {
	if os.Getenv("GTMUX_IT") == "" {
		t.Skip("integration test; run with GTMUX_IT=1 (requires tmux)")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("TMUX", "") // not inside tmux
	// Short socket dir — a unix socket path under a long $TMPDIR exceeds the ~104B
	// sun_path limit and tmux fails to start.
	sock, err := os.MkdirTemp("/tmp", "gtit")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sock)
	t.Setenv("TMUX_TMPDIR", sock) // fresh, isolated server

	script := filepath.Join(home, ".tmux/plugins/tmux-resurrect/scripts/restore.sh")
	os.MkdirAll(filepath.Dir(script), 0o755)
	os.WriteFile(script, []byte("#!/usr/bin/env bash\ntmux new-session -d -s Recovered\n"), 0o755)
	last := filepath.Join(home, ".local/share/tmux/resurrect/last")
	os.MkdirAll(filepath.Dir(last), 0o755)
	os.WriteFile(last, []byte("pane\tRecovered\t0\t0\t:\t0\tt\t:/tmp\t1\tbash\t:x\n"), 0o644)

	defer tmux.OK("kill-server")
	ensureServer()

	names := tmux.Lines("list-sessions", "-F", "#{session_name}")
	found := false
	for _, n := range names {
		if n == "Recovered" {
			found = true
		}
		if strings.HasPrefix(n, "gtmux-boot-") {
			t.Errorf("boot session not cleaned up: %v", names)
		}
	}
	if !found {
		t.Errorf("expected restored 'Recovered' session, got %v", names)
	}
}

// saveHasLayout must tell a real saved layout (window/pane lines) from an empty
// one — the check that decides whether refusing to overwrite protects work.
func TestSaveHasLayout(t *testing.T) {
	dir := t.TempDir()
	rich := filepath.Join(dir, "rich.txt")
	os.WriteFile(rich, []byte("pane\tDiting\t0\t0\t:\t0\ttitle\t:/tmp\t1\tbash\t:claude\n"), 0o644)
	if !saveHasLayout(rich) {
		t.Error("save with a pane line should report a layout")
	}
	empty := filepath.Join(dir, "empty.txt")
	os.WriteFile(empty, []byte("state\tmain\n"), 0o644)
	if saveHasLayout(empty) {
		t.Error("save with no window/pane lines is not a real layout")
	}
	if saveHasLayout(filepath.Join(dir, "missing.txt")) {
		t.Error("missing file is not a layout")
	}
}

// resurrectRestoreScript / resurrectLastSave resolve the TPM install + save
// pointer from $HOME.
func TestResurrectPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")

	if resurrectRestoreScript() != "" {
		t.Error("no script should resolve before one is installed")
	}
	script := filepath.Join(home, ".tmux/plugins/tmux-resurrect/scripts/restore.sh")
	os.MkdirAll(filepath.Dir(script), 0o755)
	os.WriteFile(script, []byte("#!/usr/bin/env bash\n"), 0o755)
	if got := resurrectRestoreScript(); got != script {
		t.Errorf("restore script = %q, want %q", got, script)
	}

	if resurrectLastSave() != "" {
		t.Error("no save should resolve before one exists")
	}
	last := filepath.Join(home, ".local/share/tmux/resurrect/last")
	os.MkdirAll(filepath.Dir(last), 0o755)
	os.WriteFile(last, []byte("x"), 0o644)
	if got := resurrectLastSave(); got != last {
		t.Errorf("last save = %q, want %q", got, last)
	}
}

// sanitizeLast must repair a `last` poisoned by an empty save (the exact failure
// that lost every session): point it at the newest save that actually has a layout,
// skipping the empty one even though the empty one is chronologically newest.
func TestSanitizeLast(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	dir := filepath.Join(home, ".local/share/tmux/resurrect")
	os.MkdirAll(dir, 0o755)

	pane := []byte("pane\tDiting\t0\t0\t:\t0\ttitle\t:/tmp\t1\tbash\t:claude\n")
	older := "tmux_resurrect_20260617T090000.txt"
	good := "tmux_resurrect_20260617T091940.txt"  // newest WITH a layout
	empty := "tmux_resurrect_20260618T112958.txt" // newest overall, but 0 bytes
	os.WriteFile(filepath.Join(dir, older), pane, 0o644)
	os.WriteFile(filepath.Join(dir, good), pane, 0o644)
	os.WriteFile(filepath.Join(dir, empty), nil, 0o644)

	last := filepath.Join(dir, "last")
	os.Symlink(empty, last) // poisoned: points at the empty save

	sanitizeLast()

	if !saveHasLayout(last) {
		t.Fatal("sanitizeLast left `last` without a layout")
	}
	target, _ := os.Readlink(last)
	if target != good {
		t.Errorf("last -> %q, want %q (newest non-empty)", target, good)
	}
}

// sanitizeLast must NOT touch a `last` that already resolves to a real layout.
func TestSanitizeLastKeepsGood(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	dir := filepath.Join(home, ".local/share/tmux/resurrect")
	os.MkdirAll(dir, 0o755)

	good := "tmux_resurrect_20260617T091940.txt"
	os.WriteFile(filepath.Join(dir, good), []byte("pane\tDiting\t0\t0\t:\t0\tt\t:/tmp\t1\tbash\t:x\n"), 0o644)
	last := filepath.Join(dir, "last")
	os.Symlink(good, last)

	sanitizeLast()

	if target, _ := os.Readlink(last); target != good {
		t.Errorf("last -> %q, want it left at %q", target, good)
	}
}

func TestSplitAttached(t *testing.T) {
	cases := []struct {
		line     string
		wantAtt  string
		wantName string
		wantOK   bool
	}{
		{"0 work", "0", "work", true},
		{"1 my session", "1", "my session", true}, // name may contain spaces
		{"0 a b c", "0", "a b c", true},
		{"noSpace", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		att, name, ok := splitAttached(c.line)
		if att != c.wantAtt || name != c.wantName || ok != c.wantOK {
			t.Errorf("splitAttached(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.line, att, name, ok, c.wantAtt, c.wantName, c.wantOK)
		}
	}
}

// keepUnattached drops sessions that gained a client since the list was built —
// the guard against opening a duplicate tab (the "two identical terminals" bug when
// a reopened terminal re-attached "HSS AI Workspace" between listing and spawning).
func TestKeepUnattached(t *testing.T) {
	list := []string{"HSS AI Workspace", "ccy-workspace", "main"}
	// "HSS AI Workspace" got attached (a reopened tab beat us) → dropped; order kept.
	live := map[string]bool{"ccy-workspace": true, "main": true}
	got := keepUnattached(list, live)
	want := []string{"ccy-workspace", "main"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	// all attached → empty; none attached → unchanged.
	if len(keepUnattached(list, map[string]bool{})) != 0 {
		t.Error("all-attached should drop everything")
	}
	all := map[string]bool{"HSS AI Workspace": true, "ccy-workspace": true, "main": true}
	if len(keepUnattached(list, all)) != 3 {
		t.Error("none-attached should keep everything")
	}
}

func TestPaneIDRe(t *testing.T) {
	match := []string{"%0", "%12", "%3.left"}
	for _, s := range match {
		if !paneIDRe.MatchString(s) {
			t.Errorf("paneIDRe should match %q", s)
		}
	}
	noMatch := []string{"work", "session", "1%", ""}
	for _, s := range noMatch {
		if paneIDRe.MatchString(s) {
			t.Errorf("paneIDRe should NOT match %q", s)
		}
	}
}

// TestShouldRecover: drive a resurrect restore into a running server only when
// the save has sessions and NONE are live (a fresh post-reboot server). If any
// saved session is already present, do nothing (normal reattach; no duplicates).
func TestShouldRecover(t *testing.T) {
	saved := []string{"Diting", "Pica", "Rodi"}
	cases := []struct {
		name string
		live map[string]bool
		want bool
	}{
		{"empty server (post-reboot)", map[string]bool{}, true},
		{"only an unrelated stray session", map[string]bool{"main": true}, true},
		{"a saved session already live", map[string]bool{"Pica": true}, false},
		{"all saved sessions live (normal reattach)", map[string]bool{"Diting": true, "Pica": true, "Rodi": true}, false},
	}
	for _, c := range cases {
		if got := shouldRecover(saved, c.live); got != c.want {
			t.Errorf("%s: shouldRecover = %v, want %v", c.name, got, c.want)
		}
	}
	if shouldRecover(nil, map[string]bool{}) {
		t.Error("no saved sessions → must not recover")
	}
}

// TestSavedSessionNames parses the session names from a resurrect save's
// window/pane lines (field 2), de-duped.
func TestSavedSessionNames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "save.txt")
	content := "window\tDiting\t0\t...\npane\tDiting\t0\t:\t...\nwindow\tPica\t1\t...\npane\tPica\t1\t:\t...\nstate\tDiting\t\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := savedSessionNames(path)
	if len(got) != 2 || got[0] != "Diting" || got[1] != "Pica" {
		t.Errorf("savedSessionNames = %v, want [Diting Pica]", got)
	}
}

// TestRestorePATH verifies the PATH built for driving restore.sh: the tmux binary's
// dir leads, the standard locations are present, the inherited PATH is appended,
// and everything is de-duplicated with no empty segments. This guards the fix for
// the "Restore from the menu bar does nothing" bug (restore.sh failing with 127
// because the tmux server's minimal PATH couldn't find tmux/tar/awk).
func TestRestorePATH(t *testing.T) {
	orig := tmux.Bin
	t.Cleanup(func() { tmux.Bin = orig })

	tmux.Bin = "/opt/homebrew/bin/tmux"
	t.Setenv("PATH", "/usr/bin:/custom:/opt/homebrew/bin")
	got := restorePATH()
	parts := strings.Split(got, ":")

	if parts[0] != "/opt/homebrew/bin" {
		t.Fatalf("PATH should lead with the tmux binary dir, got %q (full %q)", parts[0], got)
	}
	seen := map[string]int{}
	for _, p := range parts {
		if p == "" {
			t.Fatalf("empty segment in PATH %q", got)
		}
		seen[p]++
	}
	for _, want := range []string{"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/custom"} {
		if seen[want] == 0 {
			t.Fatalf("PATH missing %q: %q", want, got)
		}
	}
	for p, n := range seen {
		if n > 1 {
			t.Fatalf("PATH has duplicate %q (×%d): %q", p, n, got)
		}
	}

	// With no resolved tmux binary, the standard dirs still anchor the PATH.
	tmux.Bin = ""
	if g := restorePATH(); !strings.Contains(g, "/opt/homebrew/bin") || !strings.Contains(g, "/usr/bin") {
		t.Fatalf("PATH without tmux.Bin lost standard dirs: %q", g)
	}
}
