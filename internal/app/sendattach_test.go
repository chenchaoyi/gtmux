package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
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

func TestAttachWaitingReadsTheHooksMarker(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if attachWaiting("%7") || attachWaiting("") {
		t.Fatal("waiting without a marker")
	}
	if err := os.MkdirAll(filepath.Dir(state.WaitingPath("%7")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state.WaitingPath("%7"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !attachWaiting("%7") || attachWaiting("%8") {
		t.Fatal("the marker is per pane")
	}
}
