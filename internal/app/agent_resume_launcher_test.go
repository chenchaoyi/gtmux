package app

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// The whole point of recording a launcher is what restore TYPES. Asserting that
// resume.Command builds the right string proves the judgment, not the act — so this
// drives the real resumeAgents() against a real (isolated) tmux server and reads back
// what actually landed in the pane.
func TestRestoreResumesThroughTheRecordedLauncher(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	sock, err := os.MkdirTemp("/tmp", "gtres") // short path: a socket path caps at 104 bytes
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home) // state.Dir() keys off $HOME, and only $HOME
	t.Setenv("TMUX_TMPDIR", sock)
	t.Setenv("TMUX", "")
	t.Setenv("GTMUX_LANG", "en")
	restoreResumeFlag = "type" // type it, don't run it — the text is the evidence
	t.Cleanup(func() {
		restoreResumeFlag = ""
		_ = exec.Command("tmux", "kill-server").Run()
		os.RemoveAll(sock)
	})

	// Deliberately NARROW: the resume command is long, so this guarantees tmux wraps it.
	// The first version of this test used a wide pane and matched the raw capture, which
	// passed locally and by luck in CI until a runner with a long hostname split the
	// command mid-word ("crabs\ntub resume").
	run := exec.Command("tmux", "-f", "/dev/null", "new-session", "-d", "-s", "probe", "-x", "40", "-y", "20")
	run.Env = append(os.Environ(), "TMUX_TMPDIR="+sock, "TMUX=")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("isolated server: %v: %s", err, out)
	}

	// Isolation is not assumed. resumeAgents() types into EVERY eligible pane on the
	// ambient server, so a leak here would type into the developer's own work.
	panes := tmux.Lines("list-panes", "-a", "-F", "#{pane_id}\t#{session_name}:#{window_index}.#{pane_index}")
	if len(panes) != 1 || !strings.Contains(panes[0], "probe:") {
		t.Fatalf("not isolated — refusing to type into %d pane(s): %v", len(panes), panes)
	}
	loc := strings.SplitN(panes[0], "\t", 2)[1]

	if err := resume.Save(loc, resume.Record{
		Agent: "codex", SessionID: "sess-wrapper-1", Cwd: home,
		Launcher: "crabstub", UpdatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatal(err)
	}

	resumeAgents()

	// -J joins wrapped lines, so a command that spans the pane width is still one string.
	screen, _ := tmux.Run("capture-pane", "-J", "-p", "-t", loc)
	if !strings.Contains(screen, "crabstub resume 'sess-wrapper-1'") {
		t.Errorf("pane shows %q — restore must resume through the wrapper", strings.TrimSpace(screen))
	}
	if strings.Contains(screen, "codex resume") {
		t.Errorf("pane shows %q — still resuming by the agent's own name", strings.TrimSpace(screen))
	}
}
