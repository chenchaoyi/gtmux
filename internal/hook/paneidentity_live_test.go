package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// TestPaneFromAncestryAgainstARealServer proves the RULE against a real tmux
// server rather than against a hand-written pane table: the unit tests can only
// show that the matching logic is right about the fields we hand it, and the bug
// this fixes was a wrong belief about where those fields come from.
//
// It starts an isolated server, reads a pane's real pane_pid/pane_tty, and asks
// resolvePane to find that pane from an ancestry chain — first the pid the shell
// actually has, then the tty, then a chain belonging to neither.
func TestPaneFromAncestryAgainstARealServer(t *testing.T) {
	if tmux.Bin == "" {
		t.Skip("no tmux")
	}
	// NOT t.TempDir(): a unix socket path is capped near 104 bytes and the test
	// name in that path silently overruns it (see paneids_test.go).
	dir, err := os.MkdirTemp("/tmp", "gtx")
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

	panes := panesForIdentity()
	if len(panes) != 1 {
		t.Fatalf("probe server should hold exactly one pane, got %d", len(panes))
	}
	p := panes[0]
	if p.id == "" || p.pid == 0 || p.tty == "" {
		t.Fatalf("tmux gave an unusable pane row: %+v", p)
	}

	// The pane's own shell, as an ancestor several hops up — the shape of the
	// background-host chain that started all this.
	chain := []ancestor{{pid: 999001}, {pid: 999002}, {pid: p.pid, tty: p.tty}}
	if got := resolvePane(chain, panes); got != p.id {
		t.Fatalf("pane_pid from a live server did not resolve: got %q, want %s", got, p.id)
	}

	// tty only: the same terminal, spelled the way `ps` spells it.
	ttyOnly := []ancestor{{pid: 999003, tty: strings.TrimPrefix(p.tty, "/dev/")}}
	if got := resolvePane(ttyOnly, panes); got != p.id {
		t.Fatalf("pane_tty from a live server did not resolve: got %q, want %s", got, p.id)
	}

	// A chain that belongs to no pane must stay unidentified — this is what keeps a
	// genuinely native agent out of somebody else's row.
	stranger := []ancestor{{pid: 999004, tty: "ttys999"}}
	if got := resolvePane(stranger, panes); got != "" {
		t.Fatalf("a stranger chain claimed pane %q", got)
	}
}
