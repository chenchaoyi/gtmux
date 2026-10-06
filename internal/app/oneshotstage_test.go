package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// A one-shot goal that cannot be staged is refused before anything is created or typed.
// Any staging error used to fall back to the inline form with its whitespace collapsed,
// so "first\nsecond" went out as one joined line and was submitted with Enter (%12,
// 2026-10-06). Here $HOME/.local is a plain file, so the goals directory cannot exist,
// and tmux is a recorder: it must never be called.
func TestOneshotStagingFailureLaunchesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".local"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(t.TempDir(), "tmux-calls")
	rec := filepath.Join(t.TempDir(), "tmux")
	if err := os.WriteFile(rec, []byte("#!/bin/sh\necho \"$@\" >>"+calls+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	prev := tmux.Bin
	tmux.Bin = rec
	t.Cleanup(func() { tmux.Bin = prev })
	old := i18n.Lang()
	i18n.SetLang("en")
	t.Cleanup(func() { i18n.SetLang(old) })

	if _, _, err := oneshotCommand("claude", "", "first audit line\nsecond audit line"); err == nil {
		t.Fatal("oneshotCommand staged nothing and still returned a command")
	}
	var rc int
	errOut := captureStderr(t, func() {
		_, _, _, _, _, _, rc, _ = spawnTarget("%9", "", "", "first audit line\nsecond audit line", "claude", "", "", true, false, true, false)
	})
	if rc == 0 || !strings.Contains(errOut, "nothing was launched") {
		t.Fatalf("rc %d, stderr %q", rc, errOut)
	}
	if b, _ := os.ReadFile(calls); len(b) != 0 {
		t.Fatalf("tmux was called: %s", b)
	}
}

// A short, plain goal stays inline with its bytes as they are (it used to be
// whitespace-collapsed); one with a control character is staged instead.
func TestOneshotInlineGoalKeepsItsBytes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cmd, staged, err := oneshotCommand("claude", "", "fix  the it's test")
	if err != nil || staged != "" || !strings.HasSuffix(cmd, ` -- 'fix  the it'\''s test'`) {
		t.Fatalf("inline: cmd %q staged %q err %v", cmd, staged, err)
	}
	cmd, staged, err = oneshotCommand("claude", "", "tab\there")
	if err != nil || staged == "" || !strings.Contains(cmd, "--goal-file") {
		t.Fatalf("a tab is staged: cmd %q staged %q err %v", cmd, staged, err)
	}
	if b, _ := os.ReadFile(staged); string(b) != "tab\there" {
		t.Fatalf("staged bytes %q", b)
	}
}

// A launch line that cannot be typed into a session spawn just created is recorded, not
// dropped: spawnTarget hands back the full handle with the error, and the dispatch is
// written to the ledger undelivered (like a ready-timeout), so the session stays
// reclaimable and the next identical spawn adopts it instead of creating another (%12's
// review of #1423: two runs made two sessions and recorded nothing).
func TestOneshotLaunchFailureIsRecorded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	dir := t.TempDir()
	state := filepath.Join(dir, "state")
	rec := filepath.Join(dir, "tmux")
	script := `#!/bin/sh
case "$*" in
  *has-session*) exit 1 ;;
  *new-session*) echo created >>"` + state + `"; echo audit-created ;;
  *send-keys*) exit 7 ;;
  *pane_id*) grep -q created "` + state + `" 2>/dev/null && echo %77 ;;
  *session_name*) echo audit-created ;;
  *pane_current_command*) echo zsh ;;
esac
exit 0
`
	if err := os.WriteFile(rec, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	prev := tmux.Bin
	tmux.Bin = rec
	t.Cleanup(func() { tmux.Bin = prev })
	old := i18n.Lang()
	i18n.SetLang("en")
	t.Cleanup(func() { i18n.SetLang(old) })

	var pane, session string
	var own bool
	var launchErr error
	var rc int
	captureStderr(t, func() {
		pane, session, own, _, _, _, rc, launchErr = spawnTarget("", "", home, "first\nsecond", "claude", "", "", true, true, true, false)
	})
	if rc != 0 || launchErr == nil || pane != "%77" || session != "audit-created" || !own {
		t.Fatalf("handle: pane %q session %q own %v err %v rc %d", pane, session, own, launchErr, rc)
	}
	goals, _ := filepath.Glob(filepath.Join(home, ".local", "share", "gtmux", "dispatch", "goals", "*"))
	if len(goals) != 0 {
		t.Fatalf("an unlaunched staged goal was left behind: %v", goals)
	}

	// Through the command: the failed launch is in the ledger, undelivered, with its session.
	if err := os.Remove(state); err != nil {
		t.Fatal(err)
	}
	captureStdout(t, func() {
		captureStderr(t, func() {
			rc = cmdSpawn([]string{"--oneshot", "--agent", "claude", "--no-open", "--headless", "--cwd", home, "first\nsecond"})
		})
	})
	if rc == 0 {
		t.Fatal("a launch that could not be typed reported success")
	}
	var found bool
	for _, tk := range dispatch.ListTasks() {
		if tk.Pane == "%77" && tk.Session == "audit-created" && !tk.Delivered && tk.OwnSession {
			found = true
		}
	}
	if !found {
		t.Fatalf("no undelivered ledger entry for the created session: %+v", dispatch.ListTasks())
	}
}
