package hq

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestHQWindowTitleSurvivesAgentRenameAndLeavesOtherWindowsAlone(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("no tmux")
	}
	dir, err := os.MkdirTemp("/tmp", "gtxh") // short Unix socket path
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
	run := func(args ...string) string {
		t.Helper()
		out, err := tmux.Run(append([]string{"-f", "/dev/null"}, args...)...)
		if err != nil {
			t.Fatalf("tmux %v: %v", args, err)
		}
		return out
	}
	run("new-session", "-d", "-s", "HQ")
	t.Cleanup(func() { run("kill-server") })
	if got := run("list-sessions", "-F", "#{session_name}"); got != "HQ" {
		t.Fatalf("not an isolated server: %q", got)
	}
	run("set-option", "-g", "set-titles-string", "#S — #W")
	hqPane := run("display-message", "-p", "-t", "HQ", "#{pane_id}")
	nameHQWindow(hqPane, true)
	want := hqWindowTitle + " " + hqPane
	if got := tmux.Display(hqPane, "#{window_name}"); got != want {
		t.Fatalf("fresh HQ window = %q, want %q", got, want)
	}
	if got := tmux.Display(hqPane, "#{automatic-rename}"); got != "0" {
		t.Fatalf("HQ automatic-rename = %q, want off", got)
	}
	if got := tmux.Display(hqPane, "#{session_name} — #{window_name}"); got != "HQ — "+want {
		t.Fatalf("terminal title projection = %q", got)
	}
	// An agent changing its own pane title or running another command cannot
	// change the identity shown by tmux's window and terminal-tab formats.
	run("select-pane", "-t", hqPane, "-T", "agent's new task")
	run("set-option", "-g", "automatic-rename-format", "#{pane_title}")
	if got := tmux.Display(hqPane, "#{window_name}"); got != want {
		t.Fatalf("agent title replaced HQ identity: %q", got)
	}

	worker := run("new-window", "-t", "HQ", "-P", "-F", "#{pane_id}")
	if got := tmux.Display(worker, "#{automatic-rename}"); got != "1" {
		t.Fatalf("worker automatic-rename = %q, want its normal behavior", got)
	}
	// --here/--pane can adopt an existing automatically named window.
	adopted := run("new-window", "-t", "HQ", "-P", "-F", "#{pane_id}")
	nameHQWindow(adopted, false)
	if got := tmux.Display(adopted, "#{window_name}"); got != hqWindowTitle+" "+adopted {
		t.Fatalf("adopted HQ window = %q", got)
	}
	user := run("new-window", "-t", "HQ", "-P", "-F", "#{pane_id}")
	run("rename-window", "-t", user, "my chosen name")
	nameHQWindow(user, false)
	if got := tmux.Display(user, "#{window_name}"); got != "my chosen name" {
		t.Fatalf("explicit user name was replaced: %q", got)
	}

	releaseHQWindow(hqPane)
	if got := tmux.Display(hqPane, "#{automatic-rename}"); got != "1" {
		t.Fatalf("old HQ window remains pinned after move: %q", got)
	}
	if got := tmux.Display(hqPane, "#{window_name}"); strings.HasPrefix(got, hqWindowTitle) {
		t.Fatalf("old window still claims HQ: %q", got)
	}
}
