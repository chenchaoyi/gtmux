package hqnudge

import (
	"os"
	"path/filepath"
	"testing"
)

// A minimal HQ TUI capture with an input box. The middle line is the draft.
const (
	emptyCap   = "history\n╭──────────────╮\n│ ❯            │\n╰──────────────╯"
	draftedCap = "history\n╭──────────────╮\n│ ❯ hi there   │\n╰──────────────╯"
	// A capture with no locatable input box (structured == false).
	shellCap = "user@host ~ %"
)

type fake struct {
	box   func() string // what capture returns on each call
	sent  []string
	slept int
	nano  int64
}

func (f *fake) io() io {
	return io{
		capture: func(string) string { return f.box() },
		send:    func(_, t string) error { f.sent = append(f.sent, t); return nil },
		sleep:   func() { f.slept++ },
		nowNano: func() int64 { f.nano++; return 1_700_000_000_000_000_000 + f.nano },
	}
}

func fixed(s string) func() string { return func() string { return s } }

func queuedCount(t *testing.T) int {
	t.Helper()
	entries, _ := os.ReadDir(queueDir())
	n := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".txt" {
			n++
		}
	}
	return n
}

func TestDeliver_DraftPresent_QueuesNoSend(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{box: fixed(draftedCap)}
	deliver(f.io(), "%hq", "[gtmux] waiting %1 — hello")
	if len(f.sent) != 0 {
		t.Fatalf("a drafted box must never be typed into; sent=%v", f.sent)
	}
	if f.slept != 0 {
		t.Fatalf("a non-empty first frame must not sleep for a second; slept=%d", f.slept)
	}
	if got := queuedCount(t); got != 1 {
		t.Fatalf("nudge should be queued; queued=%d", got)
	}
}

func TestDeliver_EmptyBox_DeliversOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{box: fixed(emptyCap)}
	deliver(f.io(), "%hq", "msg-A")
	if len(f.sent) != 1 || f.sent[0] != "msg-A" {
		t.Fatalf("empty box should deliver once with the msg; sent=%v", f.sent)
	}
	if f.slept != 1 {
		t.Fatalf("empty box should be confirmed over two frames (one sleep); slept=%d", f.slept)
	}
	if got := queuedCount(t); got != 0 {
		t.Fatalf("queue should be empty after delivery; queued=%d", got)
	}
}

func TestDeliver_Coalesce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	box := draftedCap
	f := &fake{box: func() string { return box }}
	// Two nudges arrive while the user is typing → both queue, nothing sent.
	deliver(f.io(), "%hq", "A")
	deliver(f.io(), "%hq", "B")
	if len(f.sent) != 0 || queuedCount(t) != 2 {
		t.Fatalf("both should queue while drafted; sent=%v queued=%d", f.sent, queuedCount(t))
	}
	// User submits → box empty → drain coalesces into ONE line, oldest-first.
	box = emptyCap
	drain(f.io(), "%hq")
	if len(f.sent) != 1 || f.sent[0] != "A · B" {
		t.Fatalf("drain should coalesce A,B into one line; sent=%v", f.sent)
	}
	if queuedCount(t) != 0 {
		t.Fatalf("queue should be empty after drain; queued=%d", queuedCount(t))
	}
}

func TestDrain_DraftedBox_NeverEnters(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	box := draftedCap
	f := &fake{box: func() string { return box }}
	deliver(f.io(), "%hq", "queued-while-typing")
	// Queue is non-empty, but the box still holds a draft → the invariant holds.
	drain(f.io(), "%hq")
	if len(f.sent) != 0 {
		t.Fatalf("INVARIANT violated: sent into a drafted box; sent=%v", f.sent)
	}
	if queuedCount(t) != 1 {
		t.Fatalf("nudge should remain queued; queued=%d", queuedCount(t))
	}
}

func TestBoxEmpty_TwoFrameRace(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Frame 1 empty, then the user starts typing before frame 2 → NOT empty.
	frames := []string{emptyCap, draftedCap}
	i := 0
	f := &fake{box: func() string {
		s := frames[i]
		if i < len(frames)-1 {
			i++
		}
		return s
	}}
	deliver(f.io(), "%hq", "should-not-send")
	if len(f.sent) != 0 {
		t.Fatalf("a draft appearing in frame 2 must abort delivery; sent=%v", f.sent)
	}
	if queuedCount(t) != 1 {
		t.Fatalf("nudge should be queued after the aborted delivery; queued=%d", queuedCount(t))
	}
}

func TestBoxEmpty_UnstructuredIsNotEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{box: fixed(shellCap)}
	deliver(f.io(), "%hq", "no-box")
	if len(f.sent) != 0 {
		t.Fatalf("an unlocatable box must not be typed into; sent=%v", f.sent)
	}
	if queuedCount(t) != 1 {
		t.Fatalf("nudge should be queued; queued=%d", queuedCount(t))
	}
}
