package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestStatePaths pins the on-disk state contract: every path helper must derive
// from $HOME/.local/share/gtmux with the documented file/dir names. These are a
// stable interface (the hook writes them, agents/focus read them) — do not let
// them drift.
func TestStatePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".local", "share", "gtmux")

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Dir", Dir(), root},
		{"ActivePath", ActivePath("%1"), filepath.Join(root, "active", "%1")},
		{"WaitingDir", WaitingDir(), filepath.Join(root, "waiting")},
		{"WaitingPath", WaitingPath("%2"), filepath.Join(root, "waiting", "%2")},
		{"LastFinishedPath", LastFinishedPath(), filepath.Join(root, "last-finished")},
		{"IconPath", IconPath(), filepath.Join(root, "notify-icon.png")},
		{"NotifyDir", NotifyDir(), filepath.Join(root, "notify")},
		{"FrameDir", FrameDir(), filepath.Join(root, "frame")},
		{"TabOrderPath", TabOrderPath(), filepath.Join(root, "tab-order")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
			}
		})
	}
}

// TestTouchExistsRemove exercises the marker-file lifecycle: Touch creates the
// file (and any missing parent dirs), Exists observes presence, Touch is
// idempotent, and Remove deletes (tolerating an already-absent file).
func TestTouchExistsRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p := ActivePath("%9") // parent dir does not exist yet

	if Exists(p) {
		t.Fatal("marker should not exist before Touch")
	}
	if err := Touch(p); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	if !Exists(p) {
		t.Fatal("marker should exist after Touch")
	}
	// Idempotent: touching an existing marker leaves it (no error).
	if err := Touch(p); err != nil {
		t.Fatalf("second Touch: %v", err)
	}

	Remove(p)
	if Exists(p) {
		t.Fatal("marker should be gone after Remove")
	}
	// Remove of a missing file is a no-op (must not panic / surface).
	Remove(p)
}

// TestLastFinishedRoundTrip: WriteLastFinished persists a newline-terminated pane
// id; ReadLastFinished returns it trimmed. Empty when the file is absent.
func TestLastFinishedRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if got := ReadLastFinished(); got != "" {
		t.Errorf("ReadLastFinished with no file = %q, want empty", got)
	}
	if err := WriteLastFinished("%42"); err != nil {
		t.Fatalf("WriteLastFinished: %v", err)
	}
	if got := ReadLastFinished(); got != "%42" {
		t.Errorf("ReadLastFinished = %q, want %q", got, "%42")
	}
	// On-disk form is newline-terminated (the documented file shape).
	b, err := os.ReadFile(LastFinishedPath())
	if err != nil {
		t.Fatalf("read last-finished: %v", err)
	}
	if string(b) != "%42\n" {
		t.Errorf("last-finished file = %q, want %q", string(b), "%42\n")
	}
	// Overwrite replaces, doesn't append.
	if err := WriteLastFinished("%7"); err != nil {
		t.Fatalf("WriteLastFinished overwrite: %v", err)
	}
	if got := ReadLastFinished(); got != "%7" {
		t.Errorf("ReadLastFinished after overwrite = %q, want %q", got, "%7")
	}
}

// TestWaitingSet: WaitingSet returns the set of pane ids that have a marker file,
// ignoring sub-directories, and an empty (non-nil) map when the dir is absent.
func TestWaitingSet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if m := WaitingSet(); len(m) != 0 {
		t.Errorf("WaitingSet with no dir = %v, want empty", m)
	}

	if err := Touch(WaitingPath("%1")); err != nil {
		t.Fatalf("touch %%1: %v", err)
	}
	if err := Touch(WaitingPath("%2")); err != nil {
		t.Fatalf("touch %%2: %v", err)
	}
	// A sub-directory under waiting/ must be ignored (not a pane marker).
	if err := os.MkdirAll(filepath.Join(WaitingDir(), "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	got := WaitingSet()
	want := map[string]bool{"%1": true, "%2": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WaitingSet = %v, want %v", got, want)
	}
}

// TestFrameHash: the content fingerprint is deterministic, stable across calls,
// and distinguishes different content. (It need not be collision-free; we only
// compare a pane against its own prior frame.)
func TestFrameHash(t *testing.T) {
	a1 := frameHash("frame-A")
	a2 := frameHash("frame-A")
	b := frameHash("frame-B")

	if a1 != a2 {
		t.Errorf("frameHash not deterministic: %q vs %q", a1, a2)
	}
	if a1 == b {
		t.Errorf("frameHash collided on distinct content: both %q", a1)
	}
	if frameHash("") == "" {
		t.Error("frameHash of empty string should still be a non-empty hex digest")
	}
}

// TestTabOrderRoundTrip: SaveTabOrder writes one session per line; LoadTabOrder
// reads them back, dropping blank lines; empty input is ignored (never clobbers a
// good record); a missing file loads as nil.
func TestTabOrderRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if got := LoadTabOrder(); got != nil {
		t.Errorf("LoadTabOrder with no file = %v, want nil", got)
	}

	sessions := []string{"work", "side", "scratch"}
	if err := SaveTabOrder(sessions); err != nil {
		t.Fatalf("SaveTabOrder: %v", err)
	}
	if got := LoadTabOrder(); !reflect.DeepEqual(got, sessions) {
		t.Errorf("LoadTabOrder = %v, want %v", got, sessions)
	}

	// On-disk form: newline-joined + trailing newline.
	b, err := os.ReadFile(TabOrderPath())
	if err != nil {
		t.Fatalf("read tab-order: %v", err)
	}
	if string(b) != "work\nside\nscratch\n" {
		t.Errorf("tab-order file = %q", string(b))
	}

	// Empty input must NOT clobber the existing record.
	if err := SaveTabOrder(nil); err != nil {
		t.Fatalf("SaveTabOrder(nil): %v", err)
	}
	if got := LoadTabOrder(); !reflect.DeepEqual(got, sessions) {
		t.Errorf("after empty save, LoadTabOrder = %v, want unchanged %v", got, sessions)
	}
}

// TestLoadTabOrderDropsBlanks: blank and whitespace-only lines are trimmed away
// on load (the file may carry a trailing newline / stray spacing).
func TestLoadTabOrderDropsBlanks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	raw := "alpha\n\n   \nbeta\n\n"
	if err := os.WriteFile(TabOrderPath(), []byte(raw), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := LoadTabOrder()
	want := []string{"alpha", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LoadTabOrder = %v, want %v", got, want)
	}
}
