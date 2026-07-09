package state

import (
	"path/filepath"
	"testing"
)

// TestBackgroundMarker round-trips the background-work marker: write count+label,
// read it back, and clear it. A count <= 0 clears instead of writing.
func TestBackgroundMarker(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got, want := BackgroundPath("%9"), filepath.Join(home, ".local", "share", "gtmux", "bg", "%9"); got != want {
		t.Fatalf("BackgroundPath = %q, want %q", got, want)
	}

	// Absent marker → zero.
	if n, label := ReadBackground("%9"); n != 0 || label != "" {
		t.Fatalf("absent marker = (%d, %q), want (0, \"\")", n, label)
	}

	if err := WriteBackground("%9", 2, "npm run dev"); err != nil {
		t.Fatal(err)
	}
	if n, label := ReadBackground("%9"); n != 2 || label != "npm run dev" {
		t.Fatalf("marker = (%d, %q), want (2, %q)", n, label, "npm run dev")
	}

	// A tab in the label must not corrupt the "<count>\t<label>" encoding.
	if err := WriteBackground("%9", 1, "a\tb"); err != nil {
		t.Fatal(err)
	}
	if n, label := ReadBackground("%9"); n != 1 || label != "a b" {
		t.Fatalf("tab-sanitized marker = (%d, %q), want (1, %q)", n, label, "a b")
	}

	// count <= 0 clears.
	if err := WriteBackground("%9", 0, "x"); err != nil {
		t.Fatal(err)
	}
	if n, _ := ReadBackground("%9"); n != 0 {
		t.Fatalf("count 0 should clear, got %d", n)
	}

	// ClearBackground removes an existing marker.
	_ = WriteBackground("%9", 3, "go test")
	ClearBackground("%9")
	if n, _ := ReadBackground("%9"); n != 0 {
		t.Fatalf("ClearBackground should remove the marker, got %d", n)
	}
}
