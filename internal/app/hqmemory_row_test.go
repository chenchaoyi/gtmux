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

func TestHQMemoryRowDoesNotClaimAnOffMachineBackupItCannotSee(t *testing.T) {
	// gtmux is not a backup product and must not pretend to be one. What it can do is
	// refuse to let the operator assume a protection that is not there — the machine
	// this was written for had no Time Machine destination and no cloud folder, and
	// nothing said so.
	home := t.TempDir()
	t.Setenv("HOME", home)
	got := offMachineHint()
	if strings.Contains(got, "Time Machine") || strings.Contains(got, "synced") || strings.Contains(got, "同步盘") {
		t.Errorf("claimed a backup on a machine with none: %q", got)
	}

	if err := os.MkdirAll(filepath.Join(home, "Dropbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := offMachineHint(); !strings.Contains(got, "synced") && !strings.Contains(got, "同步盘") {
		t.Errorf("a synced folder was present and unmentioned: %q", got)
	}
}
