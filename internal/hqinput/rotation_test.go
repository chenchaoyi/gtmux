package hqinput

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

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
			window, err := RotationWindow("%6", 2)
			hold := window == RotationReset
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

func TestHRB005SentRotationWindowIsBoundedAndOptIn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(state.Dir(), "hqwake", "rotate-pending.json")
	if err := state.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	const reset = `{"pane":"%6","agent":"codex","retiring":"old","input":"/new","createdAt":1,"sentAt":2`
	for _, tc := range []struct {
		name, fields, pane string
		now                int64
		want               RotationOpportunity
	}{
		{"publication gap", `,"reserveBrief":true`, "%6", 3, RotationHandoff},
		{"other pane", `,"reserveBrief":true`, "%7", 3, RotationUnreserved},
		{"disabled", "", "%6", 3, RotationUnreserved},
		{"switch missing at deadline", `,"reserveBrief":true`, "%6", 2 + int64(RotationSettleWait/time.Second), RotationUnreserved},
		{"successor window", `,"reserveBrief":true,"switchedAt":4`, "%6", 46, RotationHandoff},
		{"legacy bound briefing", `,"switchedAt":4,"bootstrapAt":4`, "%6", 46, RotationHandoff},
		{"successor deadline", `,"reserveBrief":true,"switchedAt":4`, "%6", 4 + int64(RotationReadyWait/time.Second), RotationUnreserved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := state.WriteFile(path, []byte(reset+tc.fields+"}"), 0600); err != nil {
				t.Fatal(err)
			}
			if got, err := RotationWindow(tc.pane, tc.now); err != nil || got != tc.want {
				t.Fatalf("input window = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}
