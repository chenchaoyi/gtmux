package tmux

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCHH005_CommandTimeoutBoundsHungTmuxBinary(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fakeBin := filepath.Join(t.TempDir(), "fake-tmux")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	origBin := Bin
	origTimeout := CommandTimeout
	Bin = fakeBin
	CommandTimeout = 100 * time.Millisecond
	t.Cleanup(func() {
		Bin = origBin
		CommandTimeout = origTimeout
	})

	start := time.Now()
	if _, err := Run("list-panes"); err == nil {
		t.Fatal("expected timeout error from hung tmux binary")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("hung tmux Run took %v, expected bounded timeout", elapsed)
	}
	if OK("has-session") {
		t.Fatal("expected OK() = false on timed-out tmux command")
	}
}
