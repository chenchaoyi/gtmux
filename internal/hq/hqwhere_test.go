package hq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// hqWhere must name the window a person can navigate to AND the pane id every other
// gtmux command accepts — "it's running somewhere" is the answer that made this
// confusing in the first place.
func TestHQWhereNamesWindowAndPane(t *testing.T) {
	if tmux.Bin == "" {
		t.Skip("no tmux")
	}
	dir, err := os.MkdirTemp("/tmp", "gtx") // not t.TempDir(): socket paths cap near 104 bytes
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "sock")
	if err := os.MkdirAll(sock, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMUX_TMPDIR", sock)
	t.Setenv("TMUX", "")
	t.Setenv("HOME", dir)

	run := func(args ...string) string {
		out, _ := tmux.Run(append([]string{"-f", "/dev/null"}, args...)...)
		return out
	}
	run("new-session", "-d", "-s", "probe")
	t.Cleanup(func() { run("kill-server") })
	if got := run("list-sessions", "-F", "#{session_name}"); got != "probe" {
		t.Fatalf("not isolated — server holds %q", got)
	}
	pane := run("list-panes", "-a", "-F", "#{pane_id}")

	got := hqWhere(pane)
	if !strings.Contains(got, "probe:0.0") {
		t.Errorf("hqWhere(%s) = %q, should name the window (probe:0.0)", pane, got)
	}
	if !strings.Contains(got, pane) {
		t.Errorf("hqWhere(%s) = %q, should name the pane id", pane, got)
	}
}

// Nothing about this may fail the command: it is a courtesy line, not a step.
func TestHQWhereFallsBackToThePaneID(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv("TMUX", "")
	if got := hqWhere("%99"); got != "%99" {
		t.Fatalf("with no server, hqWhere = %q, want the bare pane id", got)
	}
}

// noteAtPane is best-effort by design — a missing tmux or an empty pane must be a
// no-op, never a panic or an error the caller has to handle.
func TestNoteAtPaneIsHarmlessWithoutTmux(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv("TMUX", "")
	noteAtPane("", "nothing to say to")
	noteAtPane("%99", "no such pane")
}
