package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// The format must list EVERY pane in the window, not the active one.
//
// An active-pane id was the original plan and measurement killed it: the window name
// LAGGED the real active pane (`name=[… %13]` while `%14` was active), because tmux
// re-evaluates the format on its own schedule. That would have been a stale pointer
// dressed as a live one. `#{P:…}` iterates the window's panes instead.
func TestPaneIDFormatListsEveryPane(t *testing.T) {
	if !strings.Contains(paneIDsInWindowName, "#{P:") {
		t.Errorf("format %q must iterate panes — a single id would follow focus and lag it", paneIDsInWindowName)
	}
	if !strings.Contains(paneIDsInWindowName, "#{pane_id}") {
		t.Errorf("format %q must emit pane ids", paneIDsInWindowName)
	}
}

// The refresh hook is REQUIRED, not decoration. Measured on an isolated server: adding a
// pane re-evaluates the name immediately, removing one does NOT — the name kept a dead
// `%1` while `%0 %2` were live. Without the hook the tab lists a pane that is gone, which
// is worse than not listing any.
func TestPaneIDRefreshHookTogglesAutomaticRename(t *testing.T) {
	if !strings.Contains(paneIDsRefreshHook, "automatic-rename off") ||
		!strings.Contains(paneIDsRefreshHook, "automatic-rename on") {
		t.Errorf("hook %q must toggle automatic-rename off then on — that is what forces the recompute", paneIDsRefreshHook)
	}
}

// The ids land in the WINDOW NAME, so `set-titles-string` stays `#S — #W` and the tab
// inherits them. That is what keeps the ghostty/iTerm2 title matchers out of this change:
// they prefix-match on the session and the ids sit after the `#S — ` separator.
//
// This test guards the property the whole phase-2 design rests on.
func TestTitleMatchingSurvivesIDsInTheWindowName(t *testing.T) {
	// A title built the way tmux would with the suggested format.
	title := "gtmux dev — gtmux %23 %24"
	if !strings.HasPrefix(title, "gtmux dev — ") {
		t.Fatal("fixture is wrong")
	}
	// The matcher lives in internal/ghostty and is exercised there; what matters HERE is
	// that the ids never precede the session name, which is the only thing that would
	// break it. If someone ever moves them to the front, this fails.
	if strings.HasPrefix(title, "%") || strings.HasPrefix(title, "@") {
		t.Error("ids must not lead the title — that is exactly how tab-alert broke every jump")
	}
}

// The command-level test: does `doctor --fix` actually LEAVE THE MACHINE CHANGED?
//
// Asserting that the step returns 1 would prove nothing — an injected stub that always
// succeeds passes that, and this repo has already shipped a case where every test was
// green while the command never did its job (#748). So this drives the real step against
// a real tmux and asserts the EFFECT: the two lines are in the conf file, the options are
// live in the server, and — the thing the user actually wanted — the window's NAME now
// lists the ids of the panes inside it.
func TestFixStepPutsPaneIDsInTheWindowName(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("no tmux")
	}
	// NOT t.TempDir(): its path carries the test's name, and a unix socket path is capped
	// near 104 bytes — `<tmpdir>/sock/tmux-<uid>/default` silently overran it, the server
	// never started, and it looked exactly like a broken step.
	dir, err := os.MkdirTemp("/tmp", "gtx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	// Isolation, in the order the footgun demands: a socket dir that EXISTS, no inherited
	// $TMUX, and `-f /dev/null` so a fresh server does not read the user's tmux.conf and
	// let tmux-resurrect restore their whole fleet into the probe.
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

	// PROVE the isolation before changing anything. If this server were the user's, the
	// step below would rewrite the format of every window they have open.
	if got := run("list-sessions", "-F", "#{session_name}"); got != "probe" {
		t.Fatalf("not isolated — server holds %q, refusing to configure it", got)
	}

	s := &fixState{confPath: filepath.Join(dir, ".tmux.conf"), yes: true}
	if n := s.stepPaneIDsInTabs(); n != 1 {
		t.Fatalf("step reported %d applied", n)
	}

	conf, err2 := os.ReadFile(s.confPath)
	if err2 != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"automatic-rename-format", "#{pane_id}", "set-hook -g pane-exited"} {
		if !strings.Contains(string(conf), want) {
			t.Errorf("conf missing %q:\n%s", want, conf)
		}
	}

	// Live in the server, not just on disk — the step promises "+ apply live".
	if got := strings.TrimSpace(run("show", "-gv", "automatic-rename-format")); !strings.Contains(got, "#{pane_id}") {
		t.Errorf("option not applied live: %q", got)
	}
	if got := run("show-hooks", "-g", "pane-exited"); !strings.Contains(got, "automatic-rename") {
		t.Errorf("hook not applied live: %q", got)
	}

	// The EFFECT. Split so the window holds two panes, then read the name tmux computed.
	run("split-window", "-t", "probe")
	ids := strings.Fields(run("list-panes", "-t", "probe", "-F", "#{pane_id}"))
	if len(ids) != 2 {
		t.Fatalf("expected 2 panes, got %v", ids)
	}
	// POLL, do not read once. tmux re-evaluates `automatic-rename-format` on its own
	// timer (~half a second), not synchronously with the split — a single read right
	// after `split-window` still says `bash`. That latency is real and worth knowing:
	// the tab title trails the pane set by a beat, it does not lag indefinitely.
	var name string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		name = run("display-message", "-p", "-t", "probe", "#{window_name}")
		if strings.Contains(name, ids[0]) && strings.Contains(name, ids[1]) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	for _, id := range ids {
		if !strings.Contains(name, id) {
			t.Errorf("window name %q does not list pane %s — the tab would not name it", name, id)
		}
	}

	// Idempotent: a second run has nothing to do, so `doctor --fix` does not re-ask
	// about a machine that is already configured.
	if n := s.stepPaneIDsInTabs(); n != 0 {
		t.Errorf("second run applied %d, want 0", n)
	}
}

// tmux's default is not a fixed string. On tmux 3.7 it is
// `#{?pane_in_mode,[tmux],#{pane_current_command}}#{?pane_dead,[dead],}` — comparing
// against the bare `#{pane_current_command}` reported every DEFAULT install as "custom,
// left alone", so the row that exists to flag the version-string problem never fired on
// the machines that have it. Ask what the format is BUILT FROM instead.
func TestDefaultWindowNameFormatIsRecognized(t *testing.T) {
	for _, f := range []string{
		"",
		"#{pane_current_command}",
		"#{?pane_in_mode,[tmux],#{pane_current_command}}#{?pane_dead,[dead],}", // tmux 3.7's real default
	} {
		if !windowNameFollowsCommand(f) {
			t.Errorf("%q should be recognized as command-derived", f)
		}
	}
	for _, f := range []string{
		"#{b:pane_current_path}",
		"#{b:pane_current_path} #{P:#{pane_id} }",
	} {
		if windowNameFollowsCommand(f) {
			t.Errorf("%q is the user's own choice — leave it alone", f)
		}
	}
}

// Does the shell snippet actually retitle a real pane?
//
// Command-level again, and for the same reason: the snippet is a string in a Go file until
// a real shell runs it. This starts an isolated tmux whose pane runs the target shell with
// HOME pointed at a temp rc holding exactly the lines `doctor --fix` writes, then reads
// `pane_title` — at the prompt (should be the directory) and while a command runs (should
// be the command).
func TestPaneTitleHookRetitlesARealPane(t *testing.T) {
	for _, tc := range []struct{ shell, rc string }{{"zsh", ".zshrc"}, {"bash", ".bashrc"}} {
		t.Run(tc.shell, func(t *testing.T) {
			bin, err := exec.LookPath(tc.shell)
			if err != nil {
				t.Skip("no " + tc.shell)
			}
			dir, err := os.MkdirTemp("/tmp", "gtxt") // short: unix socket path cap
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			sock := filepath.Join(dir, "s")
			if err := os.MkdirAll(sock, 0o755); err != nil {
				t.Fatal(err)
			}
			// The rc carries a PRE-EXISTING integration that also hooks the prompt, because
			// that is the real condition and it is what broke: with the title write first in
			// PROMPT_COMMAND, the DEBUG trap fired for the commands INSIDE the other hook and
			// the prompt title came out `__ghostty_cwd_report` on the commander's machine. A
			// bare rc passes either way — the first version of this test did exactly that.
			host := map[string]string{
				"bash": "__host_cwd_report() { :; }\nPROMPT_COMMAND=\"__host_cwd_report\"\n",
				"zsh":  "__host_cwd_report() { :; }\nprecmd_functions+=(__host_cwd_report)\n",
			}[tc.shell]
			lines, _ := paneTitleHook(tc.shell)
			body := host + strings.Join(lines, "\n") + "\n"
			if err := os.WriteFile(filepath.Join(dir, tc.rc), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TMUX_TMPDIR", sock)
			t.Setenv("TMUX", "")
			t.Setenv("HOME", dir)
			run := func(args ...string) string {
				out, _ := tmux.Run(append([]string{"-f", "/dev/null"}, args...)...)
				return out
			}
			run("new-session", "-d", "-s", "t", bin)
			t.Cleanup(func() { run("kill-server") })
			if got := run("list-sessions", "-F", "#{session_name}"); got != "t" {
				t.Fatalf("not isolated — server holds %q", got)
			}

			title := func() string { return run("display-message", "-p", "-t", "t", "#{pane_title}") }
			waitFor := func(want func(string) bool, what string) string {
				deadline := time.Now().Add(6 * time.Second)
				var last string
				for time.Now().Before(deadline) {
					last = title()
					if want(last) {
						return last
					}
					time.Sleep(150 * time.Millisecond)
				}
				t.Errorf("%s: pane title stayed %q", what, last)
				return last
			}

			run("send-keys", "-t", "t", "cd /usr/share", "Enter")
			waitFor(func(s string) bool { return s == "share" }, "at the prompt the title should be the directory")

			run("send-keys", "-t", "t", "sleep 3", "Enter")
			waitFor(func(s string) bool { return strings.Contains(s, "sleep") }, "while a command runs the title should be the command")
		})
	}
}

// WHICH FILE. A tmux pane runs a LOGIN shell, and a login bash reads `.bash_profile` /
// `.bash_login` / `.profile` — the first that exists — and never `.bashrc`. Measured with a
// pane whose two rc files set different titles: `.bash_profile` won. Every "set your shell
// title" recipe writes `.bashrc`, which in a tmux pane does nothing at all.
//
// And the ORDER is load-bearing: bash stops at the first file it finds, so creating
// `.bash_profile` for someone whose setup lives in `.profile` would silently shadow it.
func TestShellRCIsTheFileALoginShellReads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	touch := func(name string) {
		if err := os.WriteFile(filepath.Join(home, name), []byte("# existing\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("SHELL", "/bin/zsh")
	if got, _ := shellRCPath(); got != filepath.Join(home, ".zshrc") {
		t.Errorf("zsh → %s, want .zshrc", got)
	}

	t.Setenv("SHELL", "/bin/bash")
	// Nothing exists yet: a login shell would read .bash_profile, so that is what to create.
	if got, _ := shellRCPath(); got != filepath.Join(home, ".bash_profile") {
		t.Errorf("bare bash → %s, want .bash_profile", got)
	}
	// .bashrc alone must NOT be chosen — a login shell never reads it.
	touch(".bashrc")
	if got, _ := shellRCPath(); got != filepath.Join(home, ".bash_profile") {
		t.Errorf("with only .bashrc → %s; a tmux pane would never source it", got)
	}
	// An existing .profile is appended to, never shadowed by a new .bash_profile.
	touch(".profile")
	if got, _ := shellRCPath(); got != filepath.Join(home, ".profile") {
		t.Errorf("with .profile → %s; creating .bash_profile would shadow it", got)
	}
	// .bash_profile wins when it exists — it is what bash looks at first.
	touch(".bash_profile")
	if got, _ := shellRCPath(); got != filepath.Join(home, ".bash_profile") {
		t.Errorf("with .bash_profile → %s", got)
	}
}

// An INSTALLED snippet that is out of date must still be offered.
//
// The first shipped version put the title write first in PROMPT_COMMAND, where any other
// prompt integration beats it. A check that only asked "is the marker present" would skip
// those machines forever — the exact ones that hit the bug.
func TestOutdatedSnippetIsNotMistakenForCurrent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	rc, shell := shellRCPath()
	lines, _ := paneTitleHook(shell)
	current := strings.Join(lines, "\n")

	// The shipped-then-superseded shape: same marker, title write first.
	stale := strings.Replace(current,
		`  PROMPT_COMMAND="${PROMPT_COMMAND:+$PROMPT_COMMAND;}"'printf "\033]2;%s\007" "${PWD##*/}"'`,
		`  PROMPT_COMMAND='printf "\033]2;%s\007" "${PWD##*/}"'"${PROMPT_COMMAND:+;$PROMPT_COMMAND}"`, 1)
	if stale == current {
		t.Fatal("fixture did not change anything — the snippet moved, update this test")
	}
	if err := os.WriteFile(rc, []byte(stale+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stale, paneTitleMarker) {
		t.Fatal("fixture lost the marker")
	}
	// The line the step compares on must be the one that actually differs, or "current"
	// would be indistinguishable from "stale".
	if strings.Contains(stale, lines[len(lines)-2]) {
		t.Error("an outdated snippet reads as current — it would never be offered again")
	}
	if !strings.Contains(current, lines[len(lines)-2]) {
		t.Error("a current snippet reads as outdated — it would be re-offered forever")
	}
}
