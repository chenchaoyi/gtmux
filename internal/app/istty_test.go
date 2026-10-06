package app

import (
	"os"
	"testing"

	"github.com/creack/pty"
)

// /dev/null is a character device, which is how a file-type check mistook it
// for a terminal and made doctor ask a question nobody could answer.
func TestIsTTYAsksTheTerminalNotTheFileType(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	old := os.Stdin
	t.Cleanup(func() { os.Stdin = old })
	os.Stdin = null
	if isTTY() {
		t.Error("stdin from /dev/null counted as a terminal")
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	os.Stdin = r
	if isTTY() {
		t.Error("a pipe counted as a terminal")
	}

	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Skipf("no pty here: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()
	os.Stdin = tty
	if !isTTY() {
		t.Error("a pty is a terminal")
	}
}
