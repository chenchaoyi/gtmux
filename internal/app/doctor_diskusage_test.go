package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestRowDiskUsage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	base := state.Dir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}

	// Small footprint → OK.
	_ = os.WriteFile(filepath.Join(base, "small"), []byte("hi"), 0o644)
	if r := rowDiskUsage(); r.status != stOK {
		t.Fatalf("small state dir → stOK, got %d (%s)", r.status, r.value)
	}

	// Amber: a sparse file just over 500 MB.
	amber := filepath.Join(base, "amber.bin")
	f, _ := os.Create(amber)
	_ = f.Truncate(diskAmberBytes + (1 << 20))
	_ = f.Close()
	if r := rowDiskUsage(); r.status != stRec {
		t.Fatalf("500MB+ → stRec (amber), got %d (%s)", r.status, r.value)
	}

	// Red: grow it past 2 GB.
	f2, _ := os.OpenFile(amber, os.O_RDWR, 0o644)
	_ = f2.Truncate(diskRedBytes + (1 << 20))
	_ = f2.Close()
	if r := rowDiskUsage(); r.status != stMiss {
		t.Fatalf("2GB+ → stMiss (red), got %d (%s)", r.status, r.value)
	}
}

func TestStorageSeparatesRetainedDeliverablesFromWorkingGrowth(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	base := state.Dir()
	if err := os.MkdirAll(filepath.Join(base, "releases"), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(base, "releases", "recovery.bundle"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(diskRedBytes + 1); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := os.WriteFile(filepath.Join(base, "events"), []byte("state"), 0o600); err != nil {
		t.Fatal(err)
	}
	measured := measureStateStorage(base)
	if measured.working != 5 || measured.releases != diskRedBytes+1 || !measured.complete {
		t.Fatalf("incorrect classification: %+v", measured)
	}
	if measured.check().status != stInfo {
		t.Fatal("retained recovery must not be reported as broken working state")
	}
	measured.working = diskRedBytes + 1
	measured.largest, measured.largestBytes = "events", measured.working
	r := measured.check()
	if r.status != stMiss || !strings.Contains(r.note, "events") {
		t.Fatalf("real working growth still needs attention: %+v", r)
	}
	measured.working, measured.complete = 0, false
	if measured.check().status != stRec {
		t.Fatal("incomplete measurement must not claim a healthy state")
	}
}

func TestStorageDoesNotFollowExternalSymlinks(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "private"), []byte("not ours"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	f := measureStateStorage(root)
	if f.working != 0 || !f.complete {
		t.Fatalf("followed external data: %+v", f)
	}
}

func TestStorageMissingStateIsKnownEmpty(t *testing.T) {
	f := measureStateStorage(filepath.Join(t.TempDir(), "not-created"))
	if !f.complete || f.check().status != stOK {
		t.Fatalf("new installation is not an unreadable store: %+v", f)
	}
}

func TestStorageCategoriesIncludeBackupAndEvidence(t *testing.T) {
	root := t.TempDir()
	for name, size := range map[string]int{"logs/log": 1, "hq-snapshots/old.tar.gz": 2, "releases/package": 3, "reviews/receipt": 4, "verification/result": 5, "appstore/archive": 6, "reports/report": 7, "unknown/data": 8} {
		p := filepath.Join(root, name)
		_ = os.MkdirAll(filepath.Dir(p), 0700)
		_ = os.WriteFile(p, make([]byte, size), 0600)
	}
	f := measureStateStorage(root)
	if f.working != 1 || f.backups != 2 || f.releases != 9 || f.reports != 16 || f.unclassified != 8 {
		t.Fatalf("classification %+v", f)
	}
	if !strings.Contains(f.check().note, "hq-snapshots/") {
		t.Fatal("source and policy entry missing")
	}
}
