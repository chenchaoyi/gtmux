package hqinput

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHRW003ReservationReadsAreBoundedPrivateAndDoNotAuthorize(t *testing.T) {
	const good = `{"pane":"%6","agent":"codex","retiring":"old","input":"/new","createdAt":1}`
	for _, tc := range []struct {
		name, body string
		hold, bad  bool
	}{
		{"missing", "", false, false}, {"unsent", good, true, false},
		{"other-pane", strings.Replace(good, "%6", "%7", 1), false, false},
		{"sent", strings.TrimSuffix(good, "}") + `,"sentAt":2}`, false, false},
		{"malformed", "{", false, true}, {"oversize", strings.Repeat(" ", 8193), false, true},
		{"no-identity", `{"pane":"%6"}`, false, true},
		{"symlink", good, false, true}, {"fifo", good, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			path := filepath.Join(state.Dir(), "hqwake", "rotate-pending.json")
			if err := state.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			switch tc.name {
			case "missing":
			case "symlink":
				outside := filepath.Join(t.TempDir(), "source")
				if err := os.WriteFile(outside, []byte(tc.body), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, path); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := syscall.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
			default:
				if err := state.WriteFile(path, []byte(tc.body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, bErr := os.Lstat(path)
			hold, err := RotationPending("%6")
			if hold != tc.hold || (err != nil) != tc.bad {
				t.Fatal("reservation verdict", hold, err)
			}
			after, aErr := os.Lstat(path)
			if (aErr != nil) != (bErr != nil) || before != nil && (!os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || before.Mode() != after.Mode()) {
				t.Fatal("reservation read changed object")
			}
			if before != nil && before.Mode().IsRegular() {
				b, _ := os.ReadFile(path)
				if string(b) != tc.body {
					t.Fatal("reservation bytes changed")
				}
			}
		})
	}
}
