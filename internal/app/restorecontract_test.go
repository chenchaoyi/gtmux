package app

// The restore CONTRACT, executed (restore-contract).
//
// Restore has regressed four separate ways in a single day — lost sessions, a session
// lost twice, a pane layout flipped from stacked to side-by-side, terminal window order
// — and each previous fix pinned only the symptom someone happened to notice. That is not
// four independent bugs; it is the signature of a subsystem with no executable contract.
// Nothing here mocks tmux: the failures being chased live in the interaction between
// gtmux, tmux-resurrect and a real tmux server, which is exactly what a mock deletes.
//
// Each test states ONE dimension of "the sessions came back the way you left them", and
// runs save → kill-server → restore → assert against a real tmux on a PRIVATE server.
//
// ISOLATION (non-negotiable — this runs on a developer's own machine, where a mistake
// would destroy their live sessions and their save file): every command, and every gtmux
// call under test, is confined by two env vars set for the test process only:
//   - TMUX_TMPDIR → a temp dir, so the test's tmux server has its own socket and cannot
//     see, touch, or be confused with the user's running server.
//   - HOME        → a temp dir, so resurrect's save directory is the test's own. The
//     resurrect plugin is SYMLINKED in, so we drive the user's real installation
//     (the code path we mean to test) while writing only into the temp tree.
//
// WHAT THIS CANNOT COVER, stated rather than quietly omitted: terminal window ORDER
// (Ghostty tabs) is one of the four regressions and is NOT verifiable here — it needs a
// real terminal and the accessibility tree. It stays a manual check, and the contract
// says so instead of implying coverage it doesn't have.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// restoreEnv is an isolated tmux+resurrect world. Every helper below runs inside it.
type restoreEnv struct {
	t    *testing.T
	home string
	tmp  string
}

// newRestoreEnv builds the sandbox, or skips the test when the machine can't host it.
// It skips rather than fails: this is an integration test against tools that are not
// guaranteed present (CI has no resurrect plugin), and a skip that says WHY is more
// useful than a red build that means "not installed".
func newRestoreEnv(t *testing.T) *restoreEnv {
	t.Helper()
	if os.Getenv("GTMUX_RESTORE_E2E") == "" {
		t.Skip("restore contract: set GTMUX_RESTORE_E2E=1 to run (drives a real tmux server)")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("restore contract: tmux not installed")
	}
	realHome, homeErr := os.UserHomeDir()
	if homeErr != nil {
		t.Skip("restore contract: no home dir")
	}
	plugin := filepath.Join(realHome, ".tmux", "plugins", "tmux-resurrect")
	if fi, err := os.Stat(filepath.Join(plugin, "scripts", "restore.sh")); err != nil || fi.IsDir() {
		t.Skip("restore contract: tmux-resurrect not installed")
	}

	home := t.TempDir()
	// NOT t.TempDir(): a unix socket path is capped near 104 bytes on macOS, and the
	// per-test temp path plus tmux's own `tmux-<uid>/default` suffix blows past it
	// ("File name too long"). A short, uniquely-named dir under /tmp keeps the socket
	// addressable while staying just as isolated.
	tmp, err := os.MkdirTemp("/tmp", "gtx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	// Symlink the REAL plugin in: we want the actual restore path under test, but every
	// file it writes (the save dir) lands under the temp HOME.
	if err := os.MkdirAll(filepath.Join(home, ".tmux", "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(plugin, filepath.Join(home, ".tmux", "plugins", "tmux-resurrect")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("TMUX_TMPDIR", tmp)
	// Never inherit the outer tmux: a stray $TMUX would make tmux think we're nested.
	t.Setenv("TMUX", "")

	e := &restoreEnv{t: t, home: home, tmp: tmp}
	t.Cleanup(e.killServer)
	return e
}

// tmuxCmd runs a tmux command inside the sandbox and returns trimmed stdout.
func (e *restoreEnv) tmuxCmd(args ...string) string {
	e.t.Helper()
	c := exec.Command("tmux", args...)
	c.Env = append(os.Environ(), "HOME="+e.home, "TMUX_TMPDIR="+e.tmp, "TMUX=")
	out, err := c.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "no server running") {
		e.t.Fatalf("tmux %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (e *restoreEnv) killServer() {
	c := exec.Command("tmux", "kill-server")
	c.Env = append(os.Environ(), "HOME="+e.home, "TMUX_TMPDIR="+e.tmp, "TMUX=")
	_ = c.Run()
}

func (e *restoreEnv) serverUp() bool {
	c := exec.Command("tmux", "has-session")
	c.Env = append(os.Environ(), "HOME="+e.home, "TMUX_TMPDIR="+e.tmp, "TMUX=")
	return c.Run() == nil
}

// ── the topology we save, and the state we compare ───────────────────────────

// snapshot is the observable shape of a tmux server: everything the contract claims
// restore preserves. Compared field by field so a failure names the DIMENSION that broke,
// not "a big string differs".
type snapshot struct {
	sessions []string            // session names, sorted
	windows  map[string][]string // session → ordered "index:name"
	layouts  map[string]string   // "session:window" → tmux layout string
	cwds     map[string]string   // "session:window.pane" → path
	active   map[string]string   // session → active "window.pane"
}

func (e *restoreEnv) snap() snapshot {
	e.t.Helper()
	s := snapshot{
		windows: map[string][]string{},
		layouts: map[string]string{},
		cwds:    map[string]string{},
		active:  map[string]string{},
	}
	for _, l := range e.lines("list-sessions", "-F", "#{session_name}") {
		s.sessions = append(s.sessions, l)
	}
	sort.Strings(s.sessions)
	for _, l := range e.lines("list-windows", "-a", "-F",
		"#{session_name}\t#{window_index}\t#{window_name}\t#{window_layout}\t#{window_active}") {
		f := strings.Split(l, "\t")
		if len(f) < 5 {
			continue
		}
		sess, idx, name, layout, act := f[0], f[1], f[2], f[3], f[4]
		s.windows[sess] = append(s.windows[sess], idx+":"+name)
		s.layouts[sess+":"+idx] = layout
		if act == "1" {
			s.active[sess] = idx
		}
	}
	for _, l := range e.lines("list-panes", "-a", "-F",
		"#{session_name}:#{window_index}.#{pane_index}\t#{pane_current_path}\t#{pane_active}") {
		f := strings.Split(l, "\t")
		if len(f) < 3 {
			continue
		}
		s.cwds[f[0]] = f[1]
		if f[2] == "1" {
			loc := strings.SplitN(f[0], ":", 2)
			if len(loc) == 2 && s.active[loc[0]] == strings.SplitN(loc[1], ".", 2)[0] {
				s.active[loc[0]] = loc[1]
			}
		}
	}
	return s
}

func (e *restoreEnv) lines(args ...string) []string {
	out := e.tmuxCmd(args...)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// buildTopology creates the fixture: several sessions, multiple windows in a KNOWN order,
// and — the Pica case — a window split into stacked panes with distinct cwds.
func (e *restoreEnv) buildTopology() {
	e.t.Helper()
	dirA, dirB := e.t.TempDir(), e.t.TempDir()

	// alpha: two windows, the second split top/bottom (the layout dimension).
	e.tmuxCmd("new-session", "-d", "-s", "alpha", "-n", "one", "-c", dirA)
	e.tmuxCmd("new-window", "-t", "alpha", "-n", "two", "-c", dirA)
	e.tmuxCmd("split-window", "-v", "-t", "alpha:two", "-c", dirB) // -v = stacked
	e.tmuxCmd("split-window", "-v", "-t", "alpha:two", "-c", dirB)

	// beta / gamma: window ORDER and the session set.
	e.tmuxCmd("new-session", "-d", "-s", "beta", "-n", "first", "-c", dirB)
	e.tmuxCmd("new-window", "-t", "beta", "-n", "second", "-c", dirB)
	e.tmuxCmd("new-window", "-t", "beta", "-n", "third", "-c", dirA)
	e.tmuxCmd("new-session", "-d", "-s", "gamma", "-n", "solo", "-c", dirA)

	// A deliberate non-default active window, so "active" is a real assertion.
	e.tmuxCmd("select-window", "-t", "beta:second")
	time.Sleep(300 * time.Millisecond) // let tmux settle before the save reads it
}

// saveAndRestart saves via resurrect, kills the server, and drives gtmux's own restore.
func (e *restoreEnv) saveAndRestart() {
	e.t.Helper()
	save := filepath.Join(e.home, ".tmux", "plugins", "tmux-resurrect", "scripts", "save.sh")
	c := exec.Command("bash", save)
	c.Env = append(os.Environ(), "HOME="+e.home, "TMUX_TMPDIR="+e.tmp, "TMUX=")
	if out, err := c.CombinedOutput(); err != nil {
		e.t.Fatalf("resurrect save: %v\n%s", err, out)
	}
	if got := resurrectLastSave(); got == "" {
		e.t.Fatal("no save was produced — the fixture never got a snapshot to restore from")
	}
	e.killServer()
	if e.serverUp() {
		e.t.Fatal("server still up after kill — the test would assert against the ORIGINAL, not a restore")
	}
	ensureServer() // the code under test
}

// assertDimension reports one contract dimension, naming what broke.
func assertDimension(t *testing.T, dim string, want, got any) {
	t.Helper()
	w, g := fmt.Sprint(want), fmt.Sprint(got)
	if w != g {
		t.Errorf("restore lost %s\n  saved:    %s\n  restored: %s", dim, w, g)
	}
}

// ── the contract ─────────────────────────────────────────────────────────────

func TestRestoreContract(t *testing.T) {
	e := newRestoreEnv(t)
	e.buildTopology()
	before := e.snap()
	e.saveAndRestart()
	after := e.snap()

	t.Run("the session set comes back", func(t *testing.T) {
		assertDimension(t, "sessions", before.sessions, after.sessions)
	})

	t.Run("window order and names come back", func(t *testing.T) {
		for sess, want := range before.windows {
			assertDimension(t, "window order in session "+sess, want, after.windows[sess])
		}
	})

	// The Pica regression: a stacked split came back side-by-side. tmux encodes this in
	// the layout string, so it is exactly comparable — there is no reason it should ever
	// have been unverified.
	t.Run("pane layout comes back", func(t *testing.T) {
		for win, want := range before.layouts {
			got := after.layouts[win]
			if got == "" {
				t.Errorf("restore lost the window entirely: %s", win)
				continue
			}
			// The layout string's leading checksum varies with pane ids; compare the
			// geometry that follows it, which is what the user sees.
			assertDimension(t, "pane layout of "+win, layoutGeometry(want), layoutGeometry(got))
		}
	})

	t.Run("working directories come back", func(t *testing.T) {
		for loc, want := range before.cwds {
			assertDimension(t, "cwd of "+loc, want, after.cwds[loc])
		}
	})

	t.Run("the active window comes back", func(t *testing.T) {
		for sess, want := range before.active {
			assertDimension(t, "active pane of "+sess, want, after.active[sess])
		}
	})
}

// paneIDInLayout matches the pane id that terminates each leaf of a tmux layout string
// (`80x12,0,0,7` → the trailing `7`).
var paneIDInLayout = regexp.MustCompile(`(\d+x\d+,\d+,\d+),\d+`)

// layoutGeometry reduces a tmux layout string to the part the contract is actually
// about: the pane GEOMETRY and the bracket structure that distinguishes a stacked split
// (`[...]`) from a side-by-side one (`{...}`).
//
// Two things in the raw string legitimately differ across a restore and must not be
// compared, or the test reports a break that isn't one — which it did on its first run,
// and which would have been a false alarm shipped as a bug report:
//   - the leading CHECKSUM, derived from the whole string including pane ids;
//   - the PANE ID terminating each leaf. A restored pane is a new pane with a new id;
//     that the numbers shifted by one says nothing about whether the layout survived.
func layoutGeometry(layout string) string {
	if i := strings.Index(layout, ","); i > 0 {
		layout = layout[i+1:]
	}
	return paneIDInLayout.ReplaceAllString(layout, "$1,#")
}

// The lost-session regression, isolated: when SOME saved sessions are already live,
// recovery must still bring back the ones that are missing. gtmux's recovery skips
// entirely if any saved session is present, which would leave a missing session missing —
// the shape of "the gtmux dev session was lost twice".
func TestRestoreRecoversSessionsMissingFromARunningServer(t *testing.T) {
	e := newRestoreEnv(t)
	e.buildTopology()
	before := e.snap()

	save := filepath.Join(e.home, ".tmux", "plugins", "tmux-resurrect", "scripts", "save.sh")
	c := exec.Command("bash", save)
	c.Env = append(os.Environ(), "HOME="+e.home, "TMUX_TMPDIR="+e.tmp, "TMUX=")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("resurrect save: %v\n%s", err, out)
	}

	// A server that survived with only SOME of the saved sessions — the post-reboot
	// reality where a terminal reopened and started one, or a partial restore ran.
	e.tmuxCmd("kill-session", "-t", "beta")
	e.tmuxCmd("kill-session", "-t", "gamma")
	if got := len(e.snap().sessions); got != 1 {
		t.Fatalf("fixture: %d sessions live; want exactly the 1 survivor", got)
	}

	ensureServer()

	after := e.snap()
	assertDimension(t, "the sessions missing from a running server", before.sessions, after.sessions)
}
