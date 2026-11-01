package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveTabOrderPublishesReplacement(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveTabOrder([]string{"old"}); err != nil {
		t.Fatal(err)
	}
	// A retained inode models a reader holding the old snapshot during publication.
	old := filepath.Join(t.TempDir(), "previous")
	if err := os.Link(TabOrderPath(), old); err != nil {
		t.Fatal(err)
	}
	if err := SaveTabOrder([]string{"second", "first"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(old)
	if err != nil || string(b) != "old\n" {
		t.Fatalf("old snapshot changed: %q %v", b, err)
	}
	b, err = os.ReadFile(TabOrderPath())
	if err != nil || string(b) != "second\nfirst\n" {
		t.Fatalf("replacement: %q %v", b, err)
	}
	if mode(t, TabOrderPath()) != 0600 {
		t.Fatal("snapshot is not private")
	}
	if err := SaveTabOrder(nil); err != nil {
		t.Fatal(err)
	}
	if got := LoadTabOrder(); len(got) != 2 || got[0] != "second" {
		t.Fatalf("empty read clobbered snapshot: %v", got)
	}
}
