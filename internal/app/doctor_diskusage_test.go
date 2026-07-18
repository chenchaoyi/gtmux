package app

import (
	"os"
	"path/filepath"
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
