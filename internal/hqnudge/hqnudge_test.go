package hqnudge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// boxWith renders a minimal HQ TUI capture's input box holding draft.
func boxWith(draft string) string {
	return "╭──────────────╮\n│ ❯ " + draft + "  │\n╰──────────────╯"
}

// fake models the HQ pane: an input box over a submitted-line history. paste appends
// to the box, Enter moves the box into history — so the ack (which looks for the batch
// id in HISTORY, not the draft) is exercised for real.
type fake struct {
	inMode       bool
	unstructured bool  // the capture has no locatable input box
	pasteErr     error // paste fails
	enterErr     error // Enter fails
	swallow      bool  // Enter reports success but the line never leaves the box
	ackBlind     bool  // the ack capture cannot see history (the line scrolled away)

	draft     string   // what sits in the input box (user text and/or our paste)
	history   []string // submitted lines, oldest first
	sent      []string // payloads that actually reached history — the deliveries
	slept     int
	base      int64 // clock origin (0 → a fixed fake epoch)
	nano      int64
	onCapture func(f *fake) // mutate state between frames (the race tests)
}

func (f *fake) capture(string) string {
	if f.onCapture != nil {
		f.onCapture(f)
	}
	if f.unstructured {
		return "user@host ~ %"
	}
	return strings.Join(f.history, "\n") + "\n" + boxWith(f.draft)
}

func (f *fake) captureFull(pane string) string {
	if f.ackBlind {
		return boxWith(f.draft) // history is out of the capture's reach
	}
	return f.capture(pane)
}

func (f *fake) io() io {
	base := f.base
	if base == 0 {
		base = 1_700_000_000_000_000_000
	}
	return io{
		capture:     f.capture,
		captureFull: f.captureFull,
		paste: func(_, t string) error {
			if f.pasteErr != nil {
				return f.pasteErr
			}
			f.draft += t
			return nil
		},
		enter: func(string) error {
			if f.enterErr != nil {
				return f.enterErr
			}
			if f.swallow || f.draft == "" {
				return nil // reported success, but nothing was submitted
			}
			f.history = append(f.history, f.draft)
			f.sent = append(f.sent, f.draft)
			f.draft = ""
			return nil
		},
		sleep:   func() { f.slept++ },
		nowNano: func() int64 { f.nano++; return base + f.nano },
		inMode:  func(string) bool { return f.inMode },
	}
}

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

func claimCount(t *testing.T) int {
	t.Helper()
	entries, _ := os.ReadDir(queueDir())
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), sendingSuffix) {
			n++
		}
	}
	return n
}

// idOf returns the trailing `#<id>` of a delivered payload.
func idOf(t *testing.T, payload string) string {
	t.Helper()
	i := strings.LastIndex(payload, " · #")
	if i < 0 {
		t.Fatalf("delivered payload carries no batch id: %q", payload)
	}
	return payload[i+3:]
}

// body returns a delivered payload without its batch id.
func body(t *testing.T, payload string) string {
	t.Helper()
	i := strings.LastIndex(payload, " · #")
	if i < 0 {
		t.Fatalf("delivered payload carries no batch id: %q", payload)
	}
	return payload[:i]
}

func TestDeliver_DraftPresent_QueuesNoSend(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{draft: "hi there"}
	deliver(f.io(), "%hq", "» gtmux·waiting  %1 — hello")
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
	f := &fake{}
	deliver(f.io(), "%hq", "msg-A")
	if len(f.sent) != 1 || body(t, f.sent[0]) != "msg-A" {
		t.Fatalf("empty box should deliver once with the msg; sent=%v", f.sent)
	}
	if got := queuedCount(t); got != 0 {
		t.Fatalf("queue should be empty after a confirmed delivery; queued=%d", got)
	}
	if claimCount(t) != 0 {
		t.Fatalf("a confirmed delivery must leave no claim behind")
	}
}

func TestDeliver_Coalesce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{draft: "typing"}
	// Two nudges arrive while the user is typing → both queue, nothing sent.
	deliver(f.io(), "%hq", "A")
	deliver(f.io(), "%hq", "B")
	if len(f.sent) != 0 || queuedCount(t) != 2 {
		t.Fatalf("both should queue while drafted; sent=%v queued=%d", f.sent, queuedCount(t))
	}
	// User submits → box empty → drain coalesces into ONE line, oldest-first.
	f.draft = ""
	drain(f.io(), "%hq")
	if len(f.sent) != 1 || body(t, f.sent[0]) != "A · B" {
		t.Fatalf("drain should coalesce A,B into one line; sent=%v", f.sent)
	}
	if queuedCount(t) != 0 {
		t.Fatalf("queue should be empty after drain; queued=%d", queuedCount(t))
	}
}

func TestDrain_DraftedBox_NeverEnters(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{draft: "half-typed"}
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
	frame := 0
	f := &fake{}
	f.onCapture = func(f *fake) {
		if frame++; frame == 1 {
			f.draft = "user started typing"
		}
	}
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
	f := &fake{inMode: true} // box empty, but pane in copy-mode
	deliver(f.io(), "%hq", "» gtmux·waiting  %1")
	if len(f.sent) != 0 {
		t.Fatalf("copy-mode must not be injected into; sent=%v", f.sent)
	}
	if queuedCount(t) != 1 {
		t.Fatalf("nudge should be queued while in copy-mode; queued=%d", queuedCount(t))
	}
	// User exits copy-mode → box empty → drain delivers.
	f.inMode = false
	drain(f.io(), "%hq")
	if len(f.sent) != 1 || body(t, f.sent[0]) != "» gtmux·waiting  %1" {
		t.Fatalf("leaving copy-mode should deliver the queued nudge; sent=%v", f.sent)
	}
}

func TestBoxEmpty_UnstructuredIsNotEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{unstructured: true}
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
	f := &fake{draft: "typing"} // drafted → everything stays queued
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
	f := &fake{} // box empty — only dueness can hold it
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
	if len(f.sent) != 1 || body(t, f.sent[0]) != "merged done v3" {
		t.Fatalf("due keyed entry should flush with the NEWEST payload; sent=%v", f.sent)
	}
	if got := queuedCount(t); got != 0 {
		t.Fatalf("flushed entry must be removed; queued=%d", got)
	}
}

func TestDeliverKeyed_DueImmediatelyFlushes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{}
	deliverKeyedAt(f.io(), "%hq", "done-%14", "immediate done", 1) // due in the distant past
	if len(f.sent) != 1 || body(t, f.sent[0]) != "immediate done" {
		t.Fatalf("a past-due keyed entry delivers at once; sent=%v", f.sent)
	}
}

// ── ack + retry (hq-wake-reliability): a wake is removed only once CONFIRMED ──

func TestDrain_PasteError_Requeues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{pasteErr: errors.New("not in a mode")}
	deliver(f.io(), "%hq", "must-not-vanish")
	if queuedCount(t) != 1 || claimCount(t) != 0 {
		t.Fatalf("a failed paste must return the entry to the queue; queued=%d claimed=%d",
			queuedCount(t), claimCount(t))
	}
	if readFailCount() != 1 {
		t.Fatalf("a failed delivery should count; fails=%d", readFailCount())
	}
	// tmux recovers → the next drain delivers the SAME nudge.
	f.pasteErr = nil
	drain(f.io(), "%hq")
	if len(f.sent) != 1 || body(t, f.sent[0]) != "must-not-vanish" {
		t.Fatalf("the retried nudge should land; sent=%v", f.sent)
	}
	if readFailCount() != 0 {
		t.Fatalf("a confirmed delivery must reset the failure count; fails=%d", readFailCount())
	}
}

func TestDrain_EnterError_Requeues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{enterErr: errors.New("no such pane")}
	deliver(f.io(), "%hq", "enter-failed")
	if queuedCount(t) != 1 {
		t.Fatalf("a failed submit must keep the nudge queued; queued=%d", queuedCount(t))
	}
	if len(f.sent) != 0 {
		t.Fatalf("nothing was submitted; sent=%v", f.sent)
	}
}

// A swallowed Enter (reported success, nothing submitted) leaves the paste in the box:
// the ack finds the id in the DRAFT, not history, which is the opposite of delivered.
func TestDrain_SwallowedEnter_NotAcked(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{swallow: true}
	deliver(f.io(), "%hq", "swallowed")
	if len(f.sent) != 0 {
		t.Fatalf("nothing reached history; sent=%v", f.sent)
	}
	if queuedCount(t) != 1 {
		t.Fatalf("an unacked delivery must stay queued; queued=%d", queuedCount(t))
	}
	if readFailCount() != 1 {
		t.Fatalf("an unacked delivery counts as a failure; fails=%d", readFailCount())
	}
}

// The ack can also fail falsely — HQ's reply scrolled the line out of the capture.
// The batch is retried, and BOTH attempts carry the same id so HQ can spot the dup.
func TestDrain_UnconfirmedAck_RetriesWithTheSameID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{ackBlind: true}
	deliver(f.io(), "%hq", "» gtmux·done  %14")
	if len(f.sent) != 1 {
		t.Fatalf("the line WAS submitted; sent=%v", f.sent)
	}
	if queuedCount(t) != 1 {
		t.Fatalf("an unconfirmed batch is requeued; queued=%d", queuedCount(t))
	}
	f.ackBlind = false
	drain(f.io(), "%hq")
	if len(f.sent) != 2 {
		t.Fatalf("the batch should be re-sent; sent=%v", f.sent)
	}
	if a, b := idOf(t, f.sent[0]), idOf(t, f.sent[1]); a != b {
		t.Fatalf("a re-send must carry the SAME batch id (HQ dedups on it); %q vs %q", a, b)
	}
}

// An ack that NEVER succeeds (an agent TUI that renders submissions in a way the read
// can't confirm) must not become a re-paste loop: the entry is re-sent a bounded
// number of times — each carrying the same id, so HQ ignores the repeats — and then
// dropped, with the degradation already raised. Bounded and announced beats unbounded.
func TestDrain_UnackableEntryStopsAfterMaxAttempts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{ackBlind: true}
	deliver(f.io(), "%hq", "» gtmux·done  %14")
	for i := 1; i < maxAckAttempts+2; i++ {
		drain(f.io(), "%hq")
	}
	if len(f.sent) != maxAckAttempts {
		t.Fatalf("an unackable entry must be pasted exactly %d times; got %d",
			maxAckAttempts, len(f.sent))
	}
	ids := map[string]bool{}
	for _, s := range f.sent {
		ids[idOf(t, s)] = true
	}
	if len(ids) != 1 {
		t.Fatalf("every re-send of one batch carries the SAME id (HQ dedups on it); got %v", ids)
	}
	if queuedCount(t) != 0 || claimCount(t) != 0 {
		t.Fatalf("the entry is dropped after the last attempt; queued=%d claimed=%d",
			queuedCount(t), claimCount(t))
	}
	if !Degraded(time.Now().Unix()) {
		t.Fatal("…and the channel reads degraded — the loss must never be silent")
	}
}

// A send that ERRORS is a different animal: nothing landed, so there is no duplicate
// to fear and no reason to give up. It retries as long as it takes.
func TestDrain_SendErrorRetriesUnbounded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{pasteErr: errors.New("tmux is down")}
	deliver(f.io(), "%hq", "» gtmux·waiting  %14")
	for i := 0; i < maxAckAttempts+3; i++ {
		drain(f.io(), "%hq")
	}
	if queuedCount(t) != 1 {
		t.Fatalf("a wake nothing could deliver must still be queued; queued=%d", queuedCount(t))
	}
	f.pasteErr = nil
	drain(f.io(), "%hq")
	if len(f.sent) != 1 || body(t, f.sent[0]) != "» gtmux·waiting  %14" {
		t.Fatalf("…and lands when tmux comes back; sent=%v", f.sent)
	}
}

// A coalesced batch is capped by SIZE as well as line count: an agent TUI folds a
// large paste into a "[Pasted text +N lines]" placeholder, which would hide the very
// id the ack looks for (and a multi-kilobyte paste is the shape that fails to land).
func TestDrain_BatchCapsOnPayloadSize(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{draft: "typing"}
	x := f.io()
	long := strings.Repeat("x", 300)
	for i := 0; i < 6; i++ {
		deliver(x, "%hq", "» gtmux·done  "+long)
	}
	f.draft = ""
	drain(x, "%hq")
	if len(f.sent[0]) > maxBatchChars+len(long)+64 {
		t.Fatalf("the payload must stay near its cap; got %d chars", len(f.sent[0]))
	}
	if queuedCount(t) == 0 {
		t.Fatal("the entries that didn't fit must stay queued for the next drain")
	}
	if !strings.Contains(f.sent[0], "more queued") {
		t.Fatalf("a capped batch must say more is coming; got %q", f.sent[0])
	}
}

// The ack has to survive how a TUI actually renders a submitted line: behind its own
// marker, and WRAPPED across rows. dispatch.ContainsHead normalizes the haystack's
// whitespace, so a wrap landing between the sigil and the trailing id must not hide it.
func TestAck_MatchesAWrappedSubmittedLine(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var submitted string
	x := io{
		capture: func(string) string { return boxWith("") },
		captureFull: func(string) string {
			if submitted == "" {
				return boxWith("")
			}
			// Claude Code's transcript: "> " marker, hard-wrapped at the pane width.
			return "> " + strings.Join(wrap(submitted, 40), "\n  ") + "\n" + boxWith("")
		},
		paste: func(_, text string) error { submitted = text; return nil },
		enter: func(string) error { return nil },
		sleep: func() {},
	}
	payload := `» gtmux·done  gtmux:0.0 (%14) │ 3m │ goal:"ship the wake fix" · #a3f1c2`
	if got := deliverPayload(x, "%hq", payload, "#a3f1c2"); got != delivered {
		t.Fatalf("a wrapped submitted line must still ack; got %v", got)
	}
}

// wrap hard-wraps s at n columns, as a terminal does.
func wrap(s string, n int) []string {
	var out []string
	for r := []rune(s); len(r) > 0; {
		if len(r) > n {
			out, r = append(out, string(r[:n])), r[n:]
			continue
		}
		out, r = append(out, string(r)), nil
	}
	return out
}

func TestBatchID_DiffersForANewBatchWithTheSameText(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{}
	deliver(f.io(), "%hq", "» gtmux·resource·warn  disk 14GB free")
	deliver(f.io(), "%hq", "» gtmux·resource·warn  disk 14GB free")
	if len(f.sent) != 2 {
		t.Fatalf("both wakes should deliver; sent=%v", f.sent)
	}
	if a, b := idOf(t, f.sent[0]), idOf(t, f.sent[1]); a == b {
		t.Fatalf("two distinct wakes must not share an id (that would read as a re-send); %q", a)
	}
}

// ── orphan reclaim: a drainer that died must not strand its batch ─────────────

func stampAge(t *testing.T, name string, age time.Duration) {
	t.Helper()
	p := filepath.Join(queueDir(), name)
	old := time.Now().Add(-age)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
}

func writeClaim(t *testing.T, name, msg string, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(queueDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(queueDir(), name), []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}
	stampAge(t, name, age)
}

func TestReclaim_StrandedClaimIsDelivered(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// A drainer claimed this entry and died before delivering it.
	writeClaim(t, "1700000000000000001-9.p1.txt"+sendingSuffix, "stranded-nudge", 2*time.Minute)
	if !Pending() {
		t.Fatal("a stranded claim must read as pending — else no drain ever rescues it")
	}
	f := &fake{}
	drain(f.io(), "%hq")
	if len(f.sent) != 1 || body(t, f.sent[0]) != "stranded-nudge" {
		t.Fatalf("the stranded nudge should be reclaimed and delivered; sent=%v", f.sent)
	}
}

func TestReclaim_FreshClaimIsLeftToItsDrainer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeClaim(t, "1700000000000000001-9.p1.txt"+sendingSuffix, "in-flight", 5*time.Second)
	if Pending() {
		t.Fatal("a fresh claim belongs to a live drainer — not pending")
	}
	f := &fake{}
	drain(f.io(), "%hq")
	if len(f.sent) != 0 {
		t.Fatalf("a fresh claim must not be stolen from its drainer; sent=%v", f.sent)
	}
}

// ── priority + caps ──────────────────────────────────────────────────────────

func TestDrain_DecisionWakeOvertakesStandingWarnings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{draft: "typing"}
	x := f.io()
	deliver(x, "%hq", "» gtmux·resource·warn  disk 14GB free")
	deliver(x, "%hq", "» gtmux·limits·warn  week (fable) 93%")
	deliver(x, "%hq", "» gtmux·goal-changed  %14 │ goal:\"ship it\"")
	f.draft = ""
	drain(x, "%hq")
	if len(f.sent) != 1 {
		t.Fatalf("one coalesced delivery expected; sent=%v", f.sent)
	}
	if !strings.HasPrefix(f.sent[0], "» gtmux·goal-changed") {
		t.Fatalf("the decision-dense wake must lead the line; sent=%q", f.sent[0])
	}
}

func TestDrain_BatchCapHoldsTheRemainder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{draft: "typing"}
	x := f.io()
	for i := 0; i < maxBatchLines+4; i++ {
		deliver(x, "%hq", fmt.Sprintf("» gtmux·done  %%%d", i))
	}
	f.draft = ""
	drain(x, "%hq")
	if n := strings.Count(f.sent[0], "» gtmux·done"); n != maxBatchLines {
		t.Fatalf("a batch must cap at %d lines; got %d: %q", maxBatchLines, n, f.sent[0])
	}
	if !strings.Contains(f.sent[0], "+4 more queued") {
		t.Fatalf("a capped batch must say more is coming; got %q", f.sent[0])
	}
	if queuedCount(t) != 4 {
		t.Fatalf("the remainder stays queued; queued=%d", queuedCount(t))
	}
	drain(x, "%hq") // the next tick drains the rest
	if len(f.sent) != 2 || strings.Count(f.sent[1], "» gtmux·done") != 4 {
		t.Fatalf("the remainder should drain next; sent=%v", f.sent)
	}
}

// queuedMsgs returns every queued payload, in delivery order.
func queuedMsgs(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, n := range queuedNames(func(string) bool { return true }) {
		b, err := os.ReadFile(filepath.Join(queueDir(), n))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(b))
	}
	return out
}

func TestEvict_DropsTheLowestPriorityOldest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{draft: "typing"} // nothing drains; the queue just fills
	x := f.io()
	deliver(x, "%hq", "» gtmux·resource·warn  oldest standing")
	deliver(x, "%hq", "» gtmux·resource·warn  newest standing")
	// One write past the cap → exactly one eviction.
	for i := 0; i < maxQueueEntries-1; i++ {
		deliver(x, "%hq", fmt.Sprintf("» gtmux·goal-changed  %%%d", i))
	}
	if n := queuedCount(t); n != maxQueueEntries {
		t.Fatalf("the queue must fill to its cap and stop; queued=%d", n)
	}
	all := strings.Join(queuedMsgs(t), "\n")
	if strings.Contains(all, "oldest standing") {
		t.Fatal("the evicted entry must be the lowest-priority OLDEST one")
	}
	if !strings.Contains(all, "newest standing") {
		t.Fatal("only ONE entry should have been evicted")
	}
	if n := strings.Count(all, "» gtmux·goal-changed"); n != maxQueueEntries-1 {
		t.Fatalf("a decision-dense wake must never be evicted for a standing warning; kept %d", n)
	}
}

// A queue entry written by an older gtmux carries neither a priority field nor a
// meaningful due prefix shape — it must still drain, not strand.
func TestLegacyQueueEntry_StillDrains(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeClaim(t, "1700000000000000001-42.txt", "legacy-nudge", time.Second)
	f := &fake{}
	drain(f.io(), "%hq")
	if len(f.sent) != 1 || body(t, f.sent[0]) != "legacy-nudge" {
		t.Fatalf("a legacy-named entry must drain; sent=%v", f.sent)
	}
}

// ── degradation ──────────────────────────────────────────────────────────────

func TestDegraded_AfterConsecutiveFailures(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	f := &fake{pasteErr: errors.New("tmux is gone")}
	for i := 0; i < wakeFailLimit; i++ {
		if Degraded(now) && i < wakeFailLimit-1 {
			t.Fatalf("must not read degraded before %d failures (at %d)", wakeFailLimit, i)
		}
		deliver(f.io(), "%hq", fmt.Sprintf("» gtmux·waiting  %%%d", i))
	}
	if !Degraded(now) {
		t.Fatalf("%d consecutive unconfirmed deliveries is a degraded channel", wakeFailLimit)
	}
	f.pasteErr = nil
	drain(f.io(), "%hq")
	if Degraded(now) {
		t.Fatal("a confirmed delivery must clear the degradation")
	}
}

// The stuck-draft case: a delivery whose Enter was swallowed leaves our own paste in
// the box, so the draft guard blocks every later attempt and the failure counter can
// never climb. A due entry rotting in the queue is the signal that catches it.
func TestDegraded_OnAStuckQueue(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	writeClaim(t, "1700000000000000001-9.p0.txt", "stuck behind a draft", 20*time.Minute)
	if !Degraded(now) {
		t.Fatal("a due wake unread for 20 minutes means the channel is not landing")
	}
}

func TestDegraded_UndueEntryIsNotStuck(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	// A keyed entry inside its merge window is not due yet — it is waiting by design.
	name := fmt.Sprintf("%019d-0.p1.k-done-_14.txt", (now+60)*int64(time.Second))
	writeClaim(t, name, "merged done", 20*time.Minute)
	if Degraded(now) {
		t.Fatal("an entry still inside its merge window is not a stuck queue")
	}
}

// ── Enqueue: hold a wake when no HQ resolves right now ────────────────────────

func TestEnqueue_QueuesWithoutTyping(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	Enqueue("» gtmux·goal-changed  %14")
	if queuedCount(t) != 1 {
		t.Fatalf("Enqueue should queue the wake; queued=%d", queuedCount(t))
	}
	if !Pending() {
		t.Fatal("the queued wake must read as pending so a later drain finds it")
	}
	f := &fake{base: time.Now().UnixNano()} // Enqueue stamps a real due time
	drain(f.io(), "%hq")                    // HQ resolves later → the held wake lands
	if len(f.sent) != 1 || body(t, f.sent[0]) != "» gtmux·goal-changed  %14" {
		t.Fatalf("the held wake should land on the next drain; sent=%v", f.sent)
	}
}

// A faint suggested-next-command ghost in the HQ composer must NOT hold a nudge behind
// a phantom draft: the color-aware draft-guard drops the faint (SGR 2) text, sees an
// empty box, and delivers. A bright half-typed draft still queues (the contrast).
func TestDeliver_FaintGhostDoesNotHoldNudge(t *testing.T) {
	esc := "\x1b"
	t.Setenv("HOME", t.TempDir())

	// Ghost-only composer → delivers.
	ghost := &fake{draft: esc + "[2mping %14 that the charter needs coordinating" + esc + "[0m"}
	deliver(ghost.io(), "%hq", "» gtmux·done  %14 — landed")
	if len(ghost.sent) != 1 {
		t.Fatalf("a faint ghost box must be treated as empty and delivered; sent=%v", ghost.sent)
	}
	if !strings.Contains(ghost.sent[0], "landed") {
		t.Fatalf("the delivered payload should carry the nudge; got %q", ghost.sent[0])
	}
	if queuedCount(t) != 0 {
		t.Fatalf("nothing should remain queued after delivery; queued=%d", queuedCount(t))
	}

	// Bright half-typed draft → still queues (unchanged guard).
	t.Setenv("HOME", t.TempDir())
	real := &fake{draft: "user is half typing this"}
	deliver(real.io(), "%hq", "» gtmux·waiting  %1 — needs you")
	if len(real.sent) != 0 {
		t.Fatalf("a real bright draft must still hold the nudge; sent=%v", real.sent)
	}
	if queuedCount(t) != 1 {
		t.Fatalf("the nudge should be queued behind the real draft; queued=%d", queuedCount(t))
	}
}
