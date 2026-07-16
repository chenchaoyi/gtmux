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
	box    func() string // what capture returns on each call
	inMode bool          // pane is in tmux copy-mode
	sent   []string
	slept  int
	nano   int64
}

func (f *fake) io() io {
	return io{
		capture: func(string) string { return f.box() },
		send:    func(_, t string) error { f.sent = append(f.sent, t); return nil },
		sleep:   func() { f.slept++ },
		nowNano: func() int64 { f.nano++; return 1_700_000_000_000_000_000 + f.nano },
		inMode:  func(string) bool { return f.inMode },
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

// Copy-mode (the user is scrolling) is treated like a non-empty draft: the nudge is
// queued, never injected — so its keys can't be eaten as copy-mode nav commands.
func TestDeliver_CopyMode_Queues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{box: fixed(emptyCap), inMode: true} // box empty, but pane in copy-mode
	deliver(f.io(), "%hq", "[gtmux] waiting %1")
	if len(f.sent) != 0 {
		t.Fatalf("copy-mode must not be injected into; sent=%v", f.sent)
	}
	if queuedCount(t) != 1 {
		t.Fatalf("nudge should be queued while in copy-mode; queued=%d", queuedCount(t))
	}
	// User exits copy-mode → box empty → drain delivers.
	f.inMode = false
	drain(f.io(), "%hq")
	if len(f.sent) != 1 || f.sent[0] != "[gtmux] waiting %1" {
		t.Fatalf("leaving copy-mode should deliver the queued nudge; sent=%v", f.sent)
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

// ── keyed delivery (the per-pane done merge window, hq-perception-v2) ─────────

func TestDeliverKeyed_ReplacesInsteadOfAdding(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{box: fixed(draftedCap)} // drafted → everything stays queued
	x := f.io()
	future := int64(1_800_000_000_000_000_000) // due far in the future
	deliverKeyedAt(x, "%hq", "done-%14", "» gtmux·done  %14 │ v1", future)
	deliverKeyedAt(x, "%hq", "done-%14", "» gtmux·done  %14 │ v2", future+999)
	deliverKeyedAt(x, "%hq", "done-%14", "» gtmux·done  %14 │ v3", future+999)
	if got := queuedCount(t); got != 1 {
		t.Fatalf("same key must REPLACE, not add: queued=%d", got)
	}
	// A different key queues independently.
	deliverKeyedAt(x, "%hq", "done-%15", "» gtmux·done  %15 │ v1", future)
	if got := queuedCount(t); got != 2 {
		t.Fatalf("distinct keys are independent: queued=%d", got)
	}
}

func TestDrain_HoldsUndueKeyedEntry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{box: fixed(emptyCap)} // box empty — only dueness can hold it
	x := f.io()
	nowBase := int64(1_700_000_000_000_000_000)
	deliverKeyedAt(x, "%hq", "done-%14", "merged done v3", nowBase+int64(120e9))
	if len(f.sent) != 0 {
		t.Fatalf("an undue keyed entry must be held; sent=%v", f.sent)
	}
	if got := queuedCount(t); got != 1 {
		t.Fatalf("entry must stay queued; queued=%d", got)
	}
	// Once the fake clock passes the due time, a drain flushes it.
	f.nano = int64(121e9) // nowNano() → nowBase+121e9 > due
	drain(x, "%hq")
	if len(f.sent) != 1 || f.sent[0] != "merged done v3" {
		t.Fatalf("due keyed entry should flush with the NEWEST payload; sent=%v", f.sent)
	}
	if got := queuedCount(t); got != 0 {
		t.Fatalf("flushed entry must be removed; queued=%d", got)
	}
}

func TestDeliverKeyed_DueImmediatelyFlushes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{box: fixed(emptyCap)}
	deliverKeyedAt(f.io(), "%hq", "done-%14", "immediate done", 1) // due in the distant past
	if len(f.sent) != 1 || f.sent[0] != "immediate done" {
		t.Fatalf("a past-due keyed entry delivers at once; sent=%v", f.sent)
	}
}
