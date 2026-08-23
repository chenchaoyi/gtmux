package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// gtmux spawn turns automatic-rename OFF the instant it names a window (charter C) — and
// that is also what stops tmux's own automatic-rename-format (doctor.go's
// paneIDsInWindowName) from ever reaching that window again. Before this fix, a spawned
// window never carried its pane's %N: not the ones named from --title, and not the ones
// named from the branch/goal fallback either — both go through the same rename-window
// call. This drives the real function against a real isolated tmux and checks the EFFECT
// on both slug sources spawn can hand it, plus the two shapes the fix must not break.
func TestNameDispatchWindowBackfillsPaneIDs(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("no tmux")
	}
	// NOT t.TempDir(): its path carries the test's name and can overrun the ~104-byte unix
	// socket path cap — see paneids_test.go's TestFixStepPutsPaneIDsInTheWindowName.
	dir, err := os.MkdirTemp("/tmp", "gtxp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "s")
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
		t.Fatalf("not isolated — server holds %q, refusing to configure it", got)
	}

	t.Run("--title", func(t *testing.T) {
		pane := run("new-window", "-t", "probe", "-P", "-F", "#{pane_id}")
		slug := spawnSlug("My Task", "feat/x", "irrelevant goal")
		nameDispatchWindow(pane, slug, false)
		name := run("display-message", "-p", "-t", pane, "#{window_name}")
		if !strings.HasPrefix(name, "my-task ") {
			t.Fatalf("window name %q should keep the --title slug up front", name)
		}
		if !strings.Contains(name, pane) {
			t.Errorf("window name %q must carry its own pane id %s — this is the bug", name, pane)
		}
	})

	t.Run("no --title (branch/goal fallback slug)", func(t *testing.T) {
		pane := run("new-window", "-t", "probe", "-P", "-F", "#{pane_id}")
		slug := spawnSlug("", "feat/menubar-width", "irrelevant goal")
		nameDispatchWindow(pane, slug, false)
		name := run("display-message", "-p", "-t", pane, "#{window_name}")
		if !strings.HasPrefix(name, "menubar-width ") {
			t.Fatalf("window name %q should keep the fallback slug up front", name)
		}
		if !strings.Contains(name, pane) {
			t.Errorf("window name %q must carry its own pane id %s", name, pane)
		}
	})

	t.Run("multiple panes list every id, headless marker still leads", func(t *testing.T) {
		pane := run("new-window", "-t", "probe", "-P", "-F", "#{pane_id}")
		second := run("split-window", "-t", pane, "-P", "-F", "#{pane_id}")
		nameDispatchWindow(pane, "disk-triage", true)
		name := run("display-message", "-p", "-t", pane, "#{window_name}")
		if !strings.HasPrefix(name, headlessMarker+"disk-triage ") {
			t.Fatalf("headless marker must still lead the name: %q", name)
		}
		for _, id := range []string{pane, second} {
			if !strings.Contains(name, id) {
				t.Errorf("window name %q missing pane %s", name, id)
			}
		}
	})

	t.Run("empty slug is left alone", func(t *testing.T) {
		pane := run("new-window", "-t", "probe", "-P", "-F", "#{pane_id}")
		before := run("display-message", "-p", "-t", pane, "#{window_name}")
		nameDispatchWindow(pane, "", false)
		after := run("display-message", "-p", "-t", pane, "#{window_name}")
		if before != after {
			t.Errorf("an empty slug must not touch the window name: %q -> %q", before, after)
		}
	})
}
