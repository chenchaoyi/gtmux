package app

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// holdLab is a private tmux server whose panes show fixed screens: `cat` for a plain
// pane, and cat under the name `codex` for a pane the radar takes for Codex. Nothing in
// it is a real agent, and nothing it does reaches the user's own tmux or gtmux state.
type holdLab struct {
	t    *testing.T
	dir  string
	run  func(args ...string) string
	fake string // the fake codex binary
}

func newHoldLab(t *testing.T) *holdLab {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("no tmux")
	}
	// Short, under /tmp: a unix socket path is capped near 104 bytes.
	dir, err := os.MkdirTemp("/tmp", "gth")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "sock")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sock, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMUX_TMPDIR", sock)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("HOME", dir)
	cat, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("no cat")
	}
	// A symlink, not a copy: macOS kills a copied system binary on launch. The process
	// table names it by the link, which is what the radar reads.
	fake := filepath.Join(dir, "bin", "codex")
	if err := os.Symlink(cat, fake); err != nil {
		t.Fatal(err)
	}
	l := &holdLab{t: t, dir: dir, fake: fake}
	l.run = func(args ...string) string {
		out, _ := tmux.Run(append([]string{"-f", "/dev/null"}, args...)...)
		return out
	}
	l.run("new-session", "-d", "-s", "lab", "-x", "120", "-y", "30")
	t.Cleanup(func() { l.run("kill-server") })
	if got := l.run("list-sessions", "-F", "#{session_name}"); got != "lab" {
		t.Fatalf("not isolated — server holds %q", got)
	}
	return l
}

// pane opens a window that prints screen, then becomes prog (cat, or the fake codex).
func (l *holdLab) pane(name, screen, prog string) string {
	l.t.Helper()
	f := filepath.Join(l.dir, name+".txt")
	if err := os.WriteFile(f, []byte(screen), 0o644); err != nil {
		l.t.Fatal(err)
	}
	id := l.run("new-window", "-d", "-P", "-F", "#{pane_id}", "-t", "lab", "-n", name,
		"sh -c 'cat \"$0\"; exec \"$1\"' "+f+" "+prog)
	l.waitFor(id, func(s string) bool { return strings.Contains(s, strings.SplitN(strings.TrimSpace(screen), "\n", 2)[0]) })
	return id
}

func (l *holdLab) screen(id string) string { return l.run("capture-pane", "-p", "-t", id) }

func (l *holdLab) waitFor(id string, ok func(string) bool) {
	l.t.Helper()
	for i := 0; i < 50; i++ {
		if ok(l.screen(id)) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	l.t.Fatalf("pane %s never showed what was expected:\n%s", id, l.screen(id))
}

func radarStatus(id string) string {
	for _, p := range radar.GatherAgents() {
		if p.PaneID == id {
			return p.Status
		}
	}
	return ""
}

// The hold on real panes, with the real radar and the real screen read, and then the real
// send: what reaches each pane, not just what attachHold returns.
func TestAttachHoldOnRealPanes(t *testing.T) {
	l := newHoldLab(t)
	plain := "build finished\nall 214 tests passed\n"
	menu := l.pane("menu", claudeMenu, "cat")              // a menu, no marker, not an agent
	cx := l.pane("cxmenu", codexMenu, l.fake)              // Codex's approval menu, no marker
	fresh := l.pane("fresh", plain, l.fake)                // a quiet Codex pane, fresh marker
	stale := l.pane("stale", plain, l.fake)                // a quiet Codex pane, two-day-old draft marker
	marked := l.pane("marked", "$ ls\nREADME.md\n", "cat") // not an agent, a marker left behind
	if err := state.WriteMarker(state.WaitingPath(fresh), "permission"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{stale, marked} {
		if err := state.WriteMarker(state.WaitingPath(id), "draft"); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(state.WaitingPath(stale), old, old); err != nil {
		t.Fatal(err)
	}
	// The radar reuses a process table for two seconds; let it see every pane.
	time.Sleep(2100 * time.Millisecond)

	for _, tc := range []struct {
		name, id, radar, hold string // hold: a fragment of the reason, "" = may send
	}{
		{"a menu with no marker, on a plain pane", menu, "", "choice menu"},
		{"Codex's approval menu with no marker (review M2)", cx, "waiting", "waiting on a decision"},
		{"a fresh marker on a quiet Codex pane", fresh, "waiting", "waiting on a decision"},
		{"a stale draft marker (review L6)", stale, "idle", ""},
		{"a marker on a pane with no agent (review L6)", marked, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := radarStatus(tc.id); got != tc.radar {
				t.Fatalf("radar status = %q, want %q", got, tc.radar)
			}
			got := attachHold(tc.id)
			if (tc.hold == "") != (got == "") || !strings.Contains(got, tc.hold) {
				t.Fatalf("attachHold = %q, want %q", got, tc.hold)
			}
		})
	}
	if state.Exists(state.WaitingPath(stale)) {
		t.Fatal("the radar should have dropped the stale marker")
	}

	img := filepath.Join(l.dir, "shot.png")
	if err := os.WriteFile(img, []byte("png bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Run("send --attach to the menu: refused, nothing typed", func(t *testing.T) {
		before := l.screen(menu)
		if got := cmdSend([]string{menu, "look at this", "--attach", img}); got != 1 {
			t.Fatalf("exit %d, want 1", got)
		}
		time.Sleep(300 * time.Millisecond)
		if after := l.screen(menu); after != before {
			t.Fatalf("the pane changed:\n%s", after)
		}
	})
	// No file, no hold: answering a question is what a plain send is for. (--no-verify
	// because a bare "1" in a cat pane gives verification nothing to confirm.)
	t.Run("a plain send still answers the menu", func(t *testing.T) {
		if got := cmdSend([]string{menu, "--no-verify", "1"}); got != 0 {
			t.Fatalf("exit %d, want 0", got)
		}
		l.waitFor(menu, func(s string) bool { return strings.HasSuffix(strings.TrimSpace(s), "1") })
	})
	t.Run("send --attach past a leftover marker: delivered", func(t *testing.T) {
		if got := cmdSend([]string{marked, "look at this", "--attach", img}); got != 0 {
			t.Fatalf("exit %d, want 0", got)
		}
		l.waitFor(marked, func(s string) bool { return strings.Contains(s, "-shot.png") })
	})
}

// `send --attach --no-verify` takes the unverified paste path. When the agent starts asking
// after the paste, the refusal must say the text went in, in the words the verified path
// uses, so the menu bar (or a person) checks the input box before sending again.
//
// The pane runs cat. The menu "appears" through the screen read the hold uses: once the
// pasted note is on screen, that read also sees a permission menu. A real program printing
// the menu races the paste check itself, which is not what this pins.
func TestNoVerifyAttachSaysTheTextWasPasted(t *testing.T) {
	l := newHoldLab(t)
	id := l.run("new-window", "-d", "-P", "-F", "#{pane_id}", "-t", "lab", "-n", "nv", "cat")
	time.Sleep(300 * time.Millisecond)
	prev := holdCapture
	holdCapture = func(p string) (string, error) {
		s, err := prev(p)
		if strings.Contains(s, "look at this") {
			s += "\n" + claudeMenu
		}
		return s, err
	}
	t.Cleanup(func() { holdCapture = prev })
	img := filepath.Join(l.dir, "shot.png")
	if err := os.WriteFile(img, []byte("png bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	errOut := captureStderr(t, func() {
		code = cmdSend([]string{id, "--no-verify", "look at this", "--attach", img})
	})
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr %s\nscreen:\n%s", code, errOut, l.screen(id))
	}
	if !strings.Contains(errOut, dispatch.EvidenceHeldBeforeEnter) || !strings.Contains(errOut, "choice menu") ||
		!strings.Contains(errOut, "input box") {
		t.Fatalf("the refusal does not say the text went in: %s", errOut)
	}
	// Pasted, never submitted: cat repeats a line only once Enter ends it, so the path,
	// the last pasted line, is on screen exactly once.
	time.Sleep(300 * time.Millisecond)
	if n := strings.Count(l.screen(id), "-shot.png"); n != 1 {
		t.Fatalf("the path is on screen %d times, want 1 (no Enter):\n%s", n, l.screen(id))
	}
}

// captureStderr runs fn with os.Stderr going to a pipe and returns what it wrote.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	defer func() { os.Stderr = prev }()
	fn()
	w.Close()
	return <-done
}
