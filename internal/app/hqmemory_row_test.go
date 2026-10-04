package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hq"
)

// The row exists because the answer to "what is protecting HQ's memory" was "nothing",
// and nobody could have known: it is the one thing gtmux holds that is not reproducible.
func seedHQ(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	root := hq.MemoryRoot()
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes", "board.md"), []byte("## ① 现状"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "LOCAL.md"), []byte("the operator's own charter"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHQMemoryRowReadsItsThreeStates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := rowHQMemory(); got.status != stInfo {
		t.Errorf("a machine with no supervisor should be neutral, got %v", got.status)
	}

	seedHQ(t)
	unprotected := rowHQMemory()
	if unprotected.status != stRec {
		t.Errorf("an unprotected memory should be a recommendation, got %v", unprotected.status)
	}
	// The SIZE is the point: "backed up" and "6 MB of irreplaceable notes are backed
	// up" are different sentences to read at 2am.
	if !strings.Contains(unprotected.value, "B") {
		t.Errorf("the row does not say how much is at risk: %q", unprotected.value)
	}
	if !strings.Contains(unprotected.note, "--export") {
		t.Errorf("the row does not say what to do: %q", unprotected.note)
	}

	if _, wrote, err := hq.Snapshot(time.Now()); err != nil || !wrote {
		t.Fatalf("snapshot: wrote=%v err=%v", wrote, err)
	}
	ok := rowHQMemory()
	if ok.status != stOK {
		t.Errorf("with a snapshot the row should be OK, got %v", ok.status)
	}
	if !strings.Contains(ok.note, "1") {
		t.Errorf("the row does not count the snapshots: %q", ok.note)
	}
}
