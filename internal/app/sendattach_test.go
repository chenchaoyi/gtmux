package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/radar"
)

func TestSaveAttachmentIsNamedByContent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, err := saveAttachment("shot.png", []byte("one image"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(a) != uploadsDir() || !strings.HasSuffix(a, "-shot.png") {
		t.Fatalf("path = %s, want <uploads>/<hash>-shot.png", a)
	}
	// The same bytes come back to the same path — what lets the send interlock see a
	// retry as the same message — and the file is reused, not rewritten.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(a, old, old); err != nil {
		t.Fatal(err)
	}
	b, err := saveAttachment("shot.png", []byte("one image"))
	if err != nil || b != a {
		t.Fatalf("second save = %s, %v; want %s", b, err, a)
	}
	// Reused, and its age reset: the uploads dir is pruned by age, and an old copy must not
	// vanish right after a new send names it.
	if fi, _ := os.Stat(a); !fi.ModTime().After(old.Add(30 * time.Minute)) {
		t.Fatalf("reused copy kept its old mtime %v", fi.ModTime())
	}
	// Different bytes never land on the same path.
	c, err := saveAttachment("shot.png", []byte("another image"))
	if err != nil || c == a {
		t.Fatalf("different content got %s (err %v), same as %s", c, err, a)
	}
	if fi, err := os.Stat(c); err != nil || fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v, err %v; want 0644", fi.Mode().Perm(), err)
	}
}

func TestSaveAttachmentKeepsAnAwkwardNameInsideUploads(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, name := range []string{"Screen Shot 2026-10-04 at 00.15.png", "../../etc/passwd", "截图 1.png", ".hidden"} {
		p, err := saveAttachment(name, []byte(name))
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Dir(p) != uploadsDir() {
			t.Fatalf("%q escaped the uploads dir: %s", name, p)
		}
		if strings.ContainsAny(filepath.Base(p), " /") {
			t.Fatalf("%q kept a space or slash: %s", name, p)
		}
	}
}

func TestWithAttachmentsPutsEachPathOnItsOwnLine(t *testing.T) {
	for _, c := range []struct {
		text  string
		paths []string
		want  string
	}{
		{"look at the red box", []string{"/u/a.png"}, "look at the red box\n/u/a.png"},
		{"note\n\n", []string{"/u/a.png", "/u/b.png"}, "note\n/u/a.png\n/u/b.png"},
		{"", []string{"/u/a.png"}, "/u/a.png"},
		{"  \n", []string{"/u/a.png"}, "/u/a.png"},
		{"no files", nil, "no files"},
	} {
		if got := withAttachments(c.text, c.paths); got != c.want {
			t.Errorf("withAttachments(%q, %v) = %q, want %q", c.text, c.paths, got, c.want)
		}
	}
}

func TestAttachFilesRefusesWhatItCannotSend(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	big := filepath.Join(home, "big.bin")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxAttachBytes + 1); err != nil { // sparse: no 30 MB written
		t.Fatal(err)
	}
	f.Close()
	for name, p := range map[string]string{"missing": filepath.Join(home, "nope.png"), "a directory": home, "too large": big} {
		if _, err := attachFiles([]string{p}); err == nil {
			t.Errorf("%s: attachFiles accepted %s", name, p)
		}
	}
	ok := filepath.Join(home, "with space.png")
	if err := os.WriteFile(ok, []byte("png bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := attachFiles([]string{ok})
	if err != nil || len(got) != 1 || !strings.HasSuffix(got[0], "-with_space.png") {
		t.Fatalf("attachFiles = %v, %v", got, err)
	}
}

func TestSendAttachArgumentRules(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	img := filepath.Join(home, "shot.png")
	if err := os.WriteFile(img, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := cmdSend([]string{"%1", "--attach", img, "--key", "Enter"}); got != 2 {
		t.Fatalf("--attach with --key = %d, want 2", got)
	}
	// A pane that does not exist: refused before anything is copied.
	if got := cmdSend([]string{"%999999", "note", "--attach", img}); got != 1 {
		t.Fatalf("send to a missing pane = %d, want 1", got)
	}
	if _, err := os.Stat(uploadsDir()); !os.IsNotExist(err) {
		t.Fatalf("uploads dir created for a send that never happened (err %v)", err)
	}
}

// withStdin runs fn with os.Stdin reading s.
func withStdin(t *testing.T, s string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(s); err != nil {
		t.Fatal(err)
	}
	w.Close()
	prev := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = prev; r.Close() }()
	fn()
}

// A screenshot with no note is a message: the menu bar always sends the note on stdin,
// and an empty one was refused (exit 2) before the pane was even looked up. With an
// attachment an empty, newline or blank message is accepted, so the call gets as far as
// the (missing) pane and fails there with 1. Without one, emptiness is still refused.
func TestSendAcceptsAnEmptyNoteOnlyWithAnAttachment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	img := filepath.Join(home, "shot.png")
	if err := os.WriteFile(img, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{"", "\n", "   ", "  \n"} {
		withStdin(t, in, func() {
			if got := cmdSend([]string{"%999999", "--message-file", "-", "--attach", img}); got != 1 {
				t.Errorf("stdin %q with --attach = %d, want 1 (past the message, stopped at the pane)", in, got)
			}
		})
		withStdin(t, in, func() {
			if got := cmdSend([]string{"%999999", "--message-file", "-"}); got != 2 {
				t.Errorf("stdin %q without --attach = %d, want 2 (an empty message)", in, got)
			}
		})
	}
	// A message file that cannot be read is an error with or without an attachment.
	if got := cmdSend([]string{"%999999", "--message-file", filepath.Join(home, "missing.txt"), "--attach", img}); got != 2 {
		t.Fatalf("unreadable message file = %d, want 2", got)
	}
}

// Screens the hold must recognise, and ones it must not.
const (
	claudeMenu = "Bash command\n  rm -rf build\n\nDo you want to proceed?\n❯ 1. Yes\n  2. Yes, and don't ask again for rm commands\n  3. No, and tell Claude what to do differently (esc)\n"
	codexMenu  = "› Allow Codex to run this command?\n\n  › 1. Yes\n    2. Yes, don't ask again\n    3. No, tell Codex what to do\n"
	proseList  = "Here's the plan:\n1. First refactor the parser\n2. Then add tests\n3. Finally ship it\n\n❯ \n"
)

// stubHold gives attachHold a radar and a screen.
func stubHold(t *testing.T, rows []radar.Pane, screen string, err error) {
	t.Helper()
	prevR, prevC := holdRadar, holdCapture
	holdRadar = func() []radar.Pane { return rows }
	holdCapture = func(string) (string, error) { return screen, err }
	t.Cleanup(func() { holdRadar, holdCapture = prevR, prevC })
}

func TestAttachHoldFollowsTheRadarAndTheScreen(t *testing.T) {
	for _, tc := range []struct {
		name   string
		rows   []radar.Pane
		screen string
		err    error
		want   string // "" = may send; else a fragment of the reason
	}{
		// The radar's verdict, whatever made it: a fresh marker, Codex's ownerless approval
		// menu, a dispatch stuck at its gate.
		{"the radar says waiting", []radar.Pane{{PaneID: "%7", Status: "waiting"}}, "", nil, "waiting on a decision"},
		// A marker is still on disk while the approved tool runs; the radar shows working.
		{"the radar says working", []radar.Pane{{PaneID: "%7", Status: "working"}}, "running tests…\n", nil, ""},
		{"another pane is waiting", []radar.Pane{{PaneID: "%8", Status: "waiting"}}, "", nil, ""},
		// A menu the hook has not reported yet: the screen alone is enough.
		{"no marker, Claude's menu on screen", nil, claudeMenu, nil, "choice menu"},
		{"no marker, Codex's menu on screen", []radar.Pane{{PaneID: "%7", Status: "idle"}}, codexMenu, nil, "choice menu"},
		{"a numbered list in prose is not a menu", nil, proseList, nil, ""},
		{"a blank pane that read fine", nil, "", nil, ""},
		// Fail closed: a pane that cannot be read cannot be shown not to be asking.
		{"the pane cannot be read", nil, "", errors.New("can't find pane: %7"), "could not read the pane"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubHold(t, tc.rows, tc.screen, tc.err)
			got := attachHold("%7")
			if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
				t.Fatalf("attachHold = %q, want %q", got, tc.want)
			}
		})
	}
}

// A reused copy is only named when its refresh took. Otherwise it is written anew, and a
// copy that cannot be written fails the send instead of naming a path that will vanish.
func TestSaveAttachmentNeverNamesACopyItCouldNotKeep(t *testing.T) {
	setup := func(t *testing.T) string {
		t.Setenv("HOME", t.TempDir())
		p, err := saveAttachment("shot.png", []byte("one image"))
		if err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-8 * 24 * time.Hour)
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
		return p
	}
	fresh := func(t *testing.T, p string) {
		t.Helper()
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("named %s, which does not exist: %v", p, err)
		}
		if time.Since(fi.ModTime()) > time.Minute {
			t.Fatalf("named %s, still %s old: the pruner takes it", p, time.Since(fi.ModTime()).Round(time.Hour))
		}
		if b, _ := os.ReadFile(p); string(b) != "one image" {
			t.Fatalf("content = %q", b)
		}
	}
	swapChtimes := func(t *testing.T, f func(string, time.Time, time.Time) error) {
		prev := attachChtimes
		attachChtimes = f
		t.Cleanup(func() { attachChtimes = prev })
	}

	t.Run("the refresh fails: written anew", func(t *testing.T) {
		p := setup(t)
		swapChtimes(t, func(string, time.Time, time.Time) error { return os.ErrPermission })
		got, err := saveAttachment("shot.png", []byte("one image"))
		if err != nil || got != p {
			t.Fatalf("= %s, %v; want %s", got, err, p)
		}
		fresh(t, got)
	})

	t.Run("pruned between the read and the refresh: written anew", func(t *testing.T) {
		p := setup(t)
		swapChtimes(t, func(name string, a, m time.Time) error {
			_ = os.Remove(name) // the pruner gets there first
			return os.Chtimes(name, a, m)
		})
		got, err := saveAttachment("shot.png", []byte("one image"))
		if err != nil || got != p {
			t.Fatalf("= %s, %v; want %s", got, err, p)
		}
		fresh(t, got)
	})

	t.Run("cannot refresh and cannot rewrite: an error, not a path", func(t *testing.T) {
		p := setup(t)
		swapChtimes(t, func(string, time.Time, time.Time) error { return os.ErrPermission })
		dir := filepath.Dir(p)
		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		if got, err := saveAttachment("shot.png", []byte("one image")); err == nil {
			t.Fatalf("= %s with no error; the send would name a copy about to be pruned", got)
		}
	})

	// The reviewer's case, on a real file: an immutable old copy (chflags uchg) can be
	// neither refreshed nor replaced.
	t.Run("an immutable old copy: an error", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("chflags is macOS")
		}
		p := setup(t)
		if err := exec.Command("chflags", "uchg", p).Run(); err != nil {
			t.Skip("chflags:", err)
		}
		t.Cleanup(func() { _ = exec.Command("chflags", "nouchg", p).Run() })
		if got, err := saveAttachment("shot.png", []byte("one image")); err == nil {
			t.Fatalf("= %s with no error", got)
		}
	})
}

// The menu bar tells "refused before the paste" from "pasted, Enter withheld" by these
// evidence prefixes, and only the second gets its "check the input box" line. It keeps its
// own copy (Swift), so a reworded prefix here would silently drop that line.
func TestMenuBarKnowsWhenTheTextWasPasted(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "macapp", "Sources", "GtmuxBar", "ScreenshotSend.swift"))
	if err != nil {
		t.Skip("macapp source not available:", err)
	}
	for _, p := range []string{dispatch.EvidenceHeldBeforeEnter, dispatch.EvidenceHeldBeforeRetry} {
		if !strings.Contains(string(src), `"`+p+`"`) {
			t.Errorf("ScreenshotSend.swift does not know the prefix %q", p)
		}
	}
}
