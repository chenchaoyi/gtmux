package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
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
	// Keep the pane narrow to force line wrapping, but tall enough to retain the
	// complete wrapped command even when the test runner's temporary path is long.
	run := exec.Command("tmux", "-f", "/dev/null", "new-session", "-d", "-s", "probe", "-x", "40", "-y", "40")
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
	writeRecoveryFixture(t, "sess-wrapper-1", home)

	resumeAgents()

	// Shell redraws do not always mark every visual wrap as a continued line, even
	// with capture-pane -J. Remove capture line breaks while preserving spaces so
	// a long command split by the narrow pane still compares as one command.
	containsCommand := func(screen, command string) bool {
		screen = strings.ReplaceAll(screen, "\r", "")
		screen = strings.ReplaceAll(screen, "\n", "")
		return strings.Contains(screen, command)
	}
	// A command that spans the pane width is still one shell input; capture output
	// may split it at visual wraps.
	// The text is typed into a shell that may not have started yet; it shows once the
	// shell echoes it. Read until it does (or 5 s pass): one read straight after typing
	// saw an empty pane on a loaded CI runner (run 37371331918, a docs-only PR).
	var screen string
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		screen, _ = tmux.Run("capture-pane", "-J", "-p", "-t", loc)
		// Stop on the whole expected command (or the wrong one), never on a partial echo.
		if containsCommand(screen, "crabstub resume 'sess-wrapper-1'") || containsCommand(screen, "codex ") || time.Now().After(deadline) {
			break
		}
	}
	if !containsCommand(screen, "crabstub resume 'sess-wrapper-1'") {
		t.Errorf("pane shows %q — restore must resume through the wrapper", strings.TrimSpace(screen))
	}
	if containsCommand(screen, "codex ") {
		t.Errorf("pane shows %q — still resuming by the agent's own name", strings.TrimSpace(screen))
	}
}

func TestRestoreManagedCodexBridge(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	sock, err := os.MkdirTemp("/tmp", "gtbridge")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("TMUX_TMPDIR", sock)
	t.Setenv("TMUX", "")
	t.Setenv("GTMUX_LANG", "en")
	restoreResumeFlag = "type"
	t.Cleanup(func() {
		restoreResumeFlag = ""
		_ = exec.Command("tmux", "kill-server").Run()
		_ = os.RemoveAll(sock)
	})
	names := []string{"bridge", "fallback", "shell", "service", "headless"}
	for _, name := range names {
		if out, err := exec.Command("tmux", "-f", "/dev/null", "new-session", "-d", "-s", name, "-c", home, "-x", "160", "-y", "40").CombinedOutput(); err != nil {
			t.Fatalf("private server: %v %s", err, out)
		}
	}
	// Prove the production sender cannot reach the user's server before typing.
	canonicalSock, err := filepath.EvalSymlinks(sock)
	if err != nil {
		t.Fatal(err)
	}
	if socket := tmux.Display("", "#{socket_path}"); !strings.HasPrefix(socket, canonicalSock+"/") {
		t.Fatalf("refusing to type: foreign socket %q", socket)
	}
	if got := tmux.Lines("list-panes", "-a", "-F", "#{session_name}"); len(got) != len(names) {
		t.Fatalf("refusing to type: unexpected private panes %v", got)
	}
	lines := []string{
		savePaneCmd("bridge", "0", "0", home, "gtmux", ":/synthetic/gtmux codex -- --model model --approve-for-me resume original-thread"),
		savePaneCmd("fallback", "0", "0", home, "gtmux", ":gtmux codex -- resume fallback-thread"),
		saveShellPane("shell", "0", "0", home),
		savePaneCmd("service", "0", "0", home, "gtmux", ":gtmux serve"),
		savePaneCmd("headless", "0", "0", home, "gtmux", ":gtmux codex -- exec prompt"),
	}
	save := filepath.Join(home, ".tmux", "resurrect", "last")
	if err := os.MkdirAll(filepath.Dir(save), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(save, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bridge", "fallback"} {
		writeRecoveryFixture(t, name+"-thread", home)
	}
	for _, name := range []string{"bridge", "shell", "service", "headless"} {
		rec := resume.Record{Agent: "codex", SessionID: name + "-thread", Cwd: home, UpdatedAt: time.Now().Unix()}
		if name == "bridge" {
			// After reboot the frontend lease is dead, but its exact saved thread
			// remains restore history; it must not need live-input authority.
			rec.Frontend, rec.HomeID = "expired-frontend", transcript.CodexHomeID()
		}
		if err := resume.Save(name+":0.0", rec); err != nil {
			t.Fatal(err)
		}
	}
	if plan := buildRestorePlanFrom(save); plan.agentCount() != 2 {
		t.Fatalf("preview must match the two interactive resumes: %+v", plan.Sessions)
	}
	resumeAgents() // type only: no Codex process or model is launched
	for _, name := range names {
		var screen string
		for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(30 * time.Millisecond) {
			screen, _ = tmux.Run("capture-pane", "-J", "-p", "-t", name+":0.0")
			screen = strings.ReplaceAll(strings.ReplaceAll(screen, "\r", ""), "\n", "")
			if strings.Contains(screen, "resume '") || name != "bridge" && name != "fallback" || time.Now().After(deadline) {
				break
			}
		}
		want := name == "bridge" || name == "fallback"
		if want && (!strings.Contains(screen, " codex -- ") || !strings.Contains(screen, "resume '"+name+"-thread'")) {
			t.Fatalf("%s: original conversation was not typed through the bridge: %q", name, screen)
		}
		if !want && strings.Contains(screen, "resume '") {
			t.Fatalf("%s: stale record revived a non-interactive pane: %q", name, screen)
		}
	}
}
