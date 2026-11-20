package app

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// The real-pane fixture launches its own test binary through a link named codex.
// Keep the terminal readable for the radar and ordinary tmux input.
func TestHoldReaderChild(t *testing.T) {
	if filepath.Base(os.Args[0]) != "codex" {
		t.Skip("private pane child only")
	}
	if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
		t.Fatal(err)
	}
}
