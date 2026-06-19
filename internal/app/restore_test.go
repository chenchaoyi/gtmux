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
