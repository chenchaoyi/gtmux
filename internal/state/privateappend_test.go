package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateAppendNarrowsOwnedFilesAndRejectsSymlinks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(LogsDir(), "day.jsonl")
	if err := os.MkdirAll(LogsDir(), 0777); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(Dir(), 0777)
	_ = os.Chmod(LogsDir(), 0777)
	if err := os.WriteFile(path, []byte("old"), 0666); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(path, 0666)
	f, err := OpenPrivateAppend(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("new")
	_ = f.Close()
	for p, want := range map[string]os.FileMode{Dir(): 0700, LogsDir(): 0700, path: 0600} {
		if mode(t, p) != want {
			t.Fatalf("%s: %o", p, mode(t, p))
		}
	}
	outside := filepath.Join(t.TempDir(), "untouched")
	_ = os.WriteFile(outside, []byte("private"), 0644)
	_ = os.Chmod(outside, 0644)
	link := filepath.Join(LogsDir(), "link.jsonl")
	_ = os.Symlink(outside, link)
	if f, err := OpenPrivateAppend(link); err == nil {
		_ = f.Close()
		t.Fatal("followed leaf symlink")
	}
	raw, _ := os.ReadFile(outside)
	if string(raw) != "private" || mode(t, outside) != 0644 {
		t.Fatal("external target changed")
	}
	if f, err := OpenPrivateRead(link); err == nil {
		_ = f.Close()
		t.Fatal("read followed a symlink")
	}
	report := InspectPrivateTree(Dir())
	if report.Wide != 0 || report.Unverified != 1 {
		t.Fatalf("symlink silently certified private: %+v", report)
	}
}
