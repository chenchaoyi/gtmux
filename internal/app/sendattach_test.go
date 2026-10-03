package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if fi, _ := os.Stat(a); !fi.ModTime().Equal(old) {
		t.Fatalf("identical file was rewritten")
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
