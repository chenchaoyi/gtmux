// Package hqnudge delivers a compact event line into the live gtmux HQ pane WITHOUT
// ever clobbering or auto-submitting a half-typed draft the user is composing there —
// and without ever losing the line if the delivery fails.
//
// The bug it first fixed: nudges were injected with `send-keys … Enter`. If the user
// was mid-typing in HQ when a nudge fired, the nudge text concatenated onto the draft
// AND the trailing Enter submitted the user's half-written command. Data loss.
//
// The guard: before typing, read the HQ input box (reusing the #393 dispatch region
// detector via dispatch.DraftOf) and check the pane isn't in tmux copy-mode. Deliver
// ONLY when the box is confirmed empty over TWO frames a short interval apart and the
// pane is not scrolling; otherwise the nudge is queued to disk and NOTHING is typed. A
// queued nudge is flushed (coalesced) on the next empty box — the next injection
// attempt, HQ's own turn-end (Stop, box reliably empty), or the serve fast tick.
//
// INVARIANT: no code path here sends Enter into a non-empty HQ input box.
//
// The hook is a short-lived process per event, so the queue is PERSISTENT (a dir of
// one-file-per-nudge under state.Dir()), not in-memory. Draining claims each file with
// an atomic rename so two concurrent hooks can't double-deliver one nudge.
//
// Delivery is ACKED (hq-wake-reliability). A claim is deleted only after the batch is
// confirmed on the pane's screen; a paste/Enter error or a missing ack renames the
// claim back and a later drain retries it. A claim whose drainer died is reclaimed
// after orphanClaimAge. Because a screen is not a transactional sink, delivery is
// at-least-once: every batch carries a short `#<id>` that is STABLE across a re-send,
// so HQ can recognize (and the playbook tells it to ignore) a duplicate.
package hqnudge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// twoFrameGap is the interval between the two empty-box reads. It shrinks (cannot
// eliminate) the window where the user starts typing between the check and the send.
// It doubles as the settle gap before the delivery ack read.
const twoFrameGap = 300 * time.Millisecond

// orphanClaimAge is how long a `.sending` claim may sit before a later drain reclaims
// it. A live drain lives for a paste + Enter + one ack frame (well under a second);
// a claim older than this belongs to a drainer that DIED — the hook is a short-lived
// process and tmux can kill its pane mid-drain. Without reclaim those nudges are
// invisible to both hasPending and drainInto forever (they scan `.txt` only).
const orphanClaimAge = 60 * time.Second

// maxBatchLines and maxBatchChars cap ONE coalesced delivery. A backlog used to join
// unboundedly — and a multi-kilobyte paste is exactly the payload shape that fails to
// land (and that an agent TUI folds into a "[Pasted text +N lines]" placeholder, which
// would hide the id our ack looks for). The remainder stays queued and drains on the
// next (3s) tick, so a cap costs latency at worst, never an event.
const (
	maxBatchLines = 8
	maxBatchChars = 800
)

// maxQueueEntries bounds the queue. Past it, the lowest-priority OLDEST entry is
// evicted: a queue this deep means HQ has been away for hours, and dropping a standing
// warning that will re-fire on the next tick beats failing to paste a goal-changed.
const maxQueueEntries = 200

// wakeFailLimit is how many consecutive unconfirmed deliveries mark the channel
// degraded, and staleQueueSecs how long a due entry may sit undelivered while an HQ is
// live before the same verdict is reached (the stuck-draft case: a delivery whose Enter
// was swallowed leaves our own paste in the box, so no further attempt is even made).
const (
	wakeFailLimit  = 3
	staleQueueSecs = 10 * 60
)

// maxAckAttempts bounds how many times an entry may be PASTED without its delivery
// being confirmed. It exists because the two failure modes are not alike:
//
//   - a paste/Enter ERROR means nothing landed, so an unbounded retry is free and
//     correct — it costs no duplicate and recovers the moment tmux does;
//   - a missed ACK means the batch may well have landed and only the confirmation was
//     lost. Retrying forever would then re-paste a line HQ can already see, once per
//     drain, for as long as the queue lives.
//
// So an unconfirmed entry is re-sent at most twice more (carrying the same id, which
// the playbook tells HQ to read as a re-send) and then dropped — with wake-degraded
// already raised. A bounded, ANNOUNCED loss beats an unbounded spam loop, and beats
// the silent loss this whole change exists to end.
const maxAckAttempts = 3

// sendingSuffix marks a queue entry claimed by a drainer.
const sendingSuffix = ".sending"

// queueDir holds one file per pending HQ nudge (`<due>-<pid>.p<prio>.txt`).
func queueDir() string { return filepath.Join(state.Dir(), "hq-nudges") }

// failCountPath holds the consecutive delivery-failure counter (text int).
func failCountPath() string { return filepath.Join(queueDir(), "fail-count") }

// io is the injectable I/O surface — real tmux in production, fakes in tests.
type io struct {
	capture     func(pane string) string // visible-screen capture (the input box is at its foot)
	captureFull func(pane string) string // capture + scrollback margin (the ack read)
	paste       func(pane, text string) error
	enter       func(pane string) error
	sleep       func() // wait the two-frame interval
	nowNano     func() int64
	inMode      func(pane string) bool // pane is in tmux copy/view-mode (scrolling)
}

var prod = io{
	capture:     tmux.CapturePane,
	captureFull: tmux.CaptureFull,
	paste:       tmux.Paste,
	enter:       func(pane string) error { return tmux.SendKey(pane, "Enter") },
	sleep:       func() { time.Sleep(twoFrameGap) },
	nowNano:     func() int64 { return time.Now().UnixNano() },
	inMode:      tmux.InMode,
}

// Deliver types msg into the HQ pane, guarding a half-typed draft. msg is queued to
// disk first; if the HQ box is confirmed empty over two frames, the queue (msg plus
// any backlog) is flushed coalesced. If the box holds a draft, msg stays queued and
// NOTHING is typed. Public entry — every HQ injection goes through here.
func Deliver(pane, msg string) { deliver(prod, pane, msg) }

func deliver(x io, pane, msg string) {
	if pane == "" || strings.TrimSpace(msg) == "" {
		return
	}
	enqueue(x, msg) // to disk; nothing typed yet
	if boxEmpty(x, pane) {
		drainInto(x, pane) // one send, coalesced — this is the ONLY place we type
	}
}

// Enqueue queues msg for a LATER drain without attempting delivery — the no-HQ-right-
// now path (hq-wake-reliability): a wake whose HQ pane momentarily doesn't resolve is
// held, not dropped, and lands on the next drain that finds one. Callers gate this on
// "an HQ was seen recently" so a machine that simply runs no HQ queues nothing.
func Enqueue(msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	enqueue(prod, msg)
}

// DeliverKeyedAt queues msg under a REPLACEABLE key with a due time (unix nanos):
// a later call with the same key REPLACES the pending payload instead of adding a
// second line, and the entry is not typed before it is due. This is the per-pane
// done merge window (hq-perception-v2): rapid-fire completions of one pane collapse
// to one line carrying the newest payload, delivered when the window closes (by the
// next Deliver/Drain after dueNano). Same draft-guard invariants as Deliver.
func DeliverKeyedAt(pane, key, msg string, dueNano int64) {
	deliverKeyedAt(prod, pane, key, msg, dueNano)
}

func deliverKeyedAt(x io, pane, key, msg string, dueNano int64) {
	if pane == "" || strings.TrimSpace(msg) == "" || strings.TrimSpace(key) == "" {
		return
	}
	enqueueKeyed(x, key, msg, dueNano)
	if boxEmpty(x, pane) {
		drainInto(x, pane) // flushes whatever is DUE (this entry only once due)
	}
}

// Drain flushes any queued nudges into the HQ pane when its box is empty (coalesced).
// Called on HQ's own turn-end (box reliably empty) and on the serve fast tick, so a
// queued nudge lands even with no further Deliver call. No-op on a drafted box or an
// empty queue.
func Drain(pane string) { drain(prod, pane) }

func drain(x io, pane string) {
	if pane == "" || !hasPending() {
		return // cheap gate: skip the capture/sleep when nothing is queued
	}
	if boxEmpty(x, pane) {
		drainInto(x, pane)
	}
}

// Pending reports whether any nudge is queued behind a draft. Cheap (a dir scan, no
// tmux) so callers can gate the more expensive Drain (which captures the pane) on it.
// A stranded claim counts as pending — otherwise the gate would hide the very entries
// the orphan reclaim exists to rescue.
func Pending() bool { return hasPending() }

func hasPending() bool {
	for _, e := range readQueue() {
		if strings.HasSuffix(e.Name(), ".txt") || isOrphanClaim(e) {
			return true
		}
	}
	return false
}

// Degraded reports whether the wake channel is failing to reach HQ: either
// wakeFailLimit consecutive deliveries went unconfirmed, or a due entry has been stuck
// in the queue past staleQueueSecs (a swallowed Enter leaves our own paste in the box,
// which blocks every later attempt at the draft guard — so the failure counter alone
// would never climb). Callers gate on a live HQ pane: no HQ means nothing to wake, not
// a degradation. now is unix seconds.
func Degraded(now int64) bool {
	if readFailCount() >= wakeFailLimit {
		return true
	}
	oldest := int64(0)
	for _, e := range readQueue() {
		if !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		if dueOf(e.Name()) > now*int64(time.Second) {
			continue // still inside its merge window — not stuck, just not due
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if ts := info.ModTime().Unix(); oldest == 0 || ts < oldest {
			oldest = ts
		}
	}
	return oldest > 0 && now-oldest > staleQueueSecs
}

// readQueue lists the queue dir (empty on any error — an absent dir is an empty queue).
func readQueue() []os.DirEntry {
	entries, _ := os.ReadDir(queueDir())
	out := entries[:0]
	for _, e := range entries {
		if !e.IsDir() && e.Name() != filepath.Base(failCountPath()) {
			out = append(out, e)
		}
	}
	return out
}

// isOrphanClaim reports whether e is a `.sending` claim old enough that its drainer
// must be gone.
func isOrphanClaim(e os.DirEntry) bool {
	if !strings.HasSuffix(e.Name(), sendingSuffix) {
		return false
	}
	info, err := e.Info()
	return err == nil && time.Since(info.ModTime()) > orphanClaimAge
}

// reclaimOrphans returns stranded claims to the queue. Run at the head of every drain.
// A reclaim can race a live-but-slow drainer, in which case the batch delivers twice —
// with the SAME batch id, which is the documented (and recognizable) duplicate. The
// alternative it replaces was a permanent silent loss.
func reclaimOrphans() {
	for _, e := range readQueue() {
		if !isOrphanClaim(e) {
			continue
		}
		src := filepath.Join(queueDir(), e.Name())
		_ = os.Rename(src, strings.TrimSuffix(src, sendingSuffix))
	}
}

// boxEmpty reports whether the HQ input box is empty, confirmed over TWO frames. A
// non-empty first frame returns false immediately (no sleep, no send). A capture with
// no locatable input region (structured == false) is treated as NOT empty — we only
// ever type into a confirmed-empty, structured box.
func boxEmpty(x io, pane string) bool {
	// Copy/view-mode: the user is scrolling, and injected keys are eaten as copy-mode
	// NAV commands (`f` → jump-forward, yellow residue) instead of reaching the box.
	// Treat it exactly like a non-empty draft — do not inject; queue, deliver on exit.
	if x.inMode != nil && x.inMode(pane) {
		return false
	}
	if draft, structured := dispatch.DraftOf(x.capture(pane)); !structured || strings.TrimSpace(draft) != "" {
		return false
	}
	x.sleep()
	draft, structured := dispatch.DraftOf(x.capture(pane))
	return structured && strings.TrimSpace(draft) == ""
}

// enqueue writes one pending-nudge file. The name is time-ordered (fixed-width nanos)
// so a lexical sort on drain is oldest-first; the pid suffix avoids collisions between
// concurrent hook processes, and the `.p<n>` field carries the wake class's priority.
func enqueue(x io, msg string) {
	if err := os.MkdirAll(queueDir(), 0o755); err != nil {
		return
	}
	evictOverflow()
	name := fmt.Sprintf("%019d-%d.p%d.txt", x.nowNano(), os.Getpid(), hqwake.PriorityOf(msg))
	_ = os.WriteFile(filepath.Join(queueDir(), name), []byte(msg), 0o644)
}

// enqueueKeyed writes/REPLACES the pending entry for key. The name embeds the due
// time in the same fixed-width-nanos prefix (so lexical order stays time order and
// drain's due check can read it back); a replacement keeps the ORIGINAL due (window
// position) and only swaps the payload for the newest one.
func enqueueKeyed(x io, key, msg string, dueNano int64) {
	if err := os.MkdirAll(queueDir(), 0o755); err != nil {
		return
	}
	// The field is matched with its trailing dot rather than `.txt`, so an entry that
	// has been re-sent (`.k-<key>.a1.txt`) is still this key's slot.
	field := keyField(key) + "."
	for _, e := range readQueue() {
		if strings.Contains(e.Name(), field) && strings.HasSuffix(e.Name(), ".txt") {
			// Replace in place: newest payload, original due position.
			_ = os.WriteFile(filepath.Join(queueDir(), e.Name()), []byte(msg), 0o644)
			return
		}
	}
	evictOverflow()
	name := fmt.Sprintf("%019d-0.p%d%s.txt", dueNano, hqwake.PriorityOf(msg), keyField(key))
	_ = os.WriteFile(filepath.Join(queueDir(), name), []byte(msg), 0o644)
}

// evictOverflow keeps the queue under maxQueueEntries by dropping the LOWEST-priority
// OLDEST entry (never a claim in flight). Deliberate priority inversion: a standing
// resource warning re-fires on the next tick, a goal-changed never does — and the
// oldest standing warning is also the most stale reading of the two.
func evictOverflow() {
	names := queuedNames(func(string) bool { return true })
	for len(names) >= maxQueueEntries {
		i := len(names) - 1
		for lowest := prioOf(names[i]); i > 0 && prioOf(names[i-1]) == lowest; i-- {
		}
		_ = os.Remove(filepath.Join(queueDir(), names[i]))
		names = append(names[:i], names[i+1:]...)
	}
}

// queuedNames returns the `.txt` entry names passing keep, ordered for delivery:
// highest priority first, oldest first within a priority.
func queuedNames(keep func(name string) bool) []string {
	var names []string
	for _, e := range readQueue() {
		if strings.HasSuffix(e.Name(), ".txt") && keep(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // oldest-first (fixed-width nanos prefix)
	sort.SliceStable(names, func(i, j int) bool { return prioOf(names[i]) < prioOf(names[j]) })
	return names
}

// keyField renders a merge key as its filename field.
func keyField(key string) string { return ".k-" + sanitizeKey(key) }

// sanitizeKey makes a queue-filename-safe key (pane ids like "%14" pass through;
// path separators cannot). Dots go too: they separate the name's OWN fields, so a
// dotted key could otherwise be misread as a priority or an attempt counter.
func sanitizeKey(key string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', '.', 0:
			return '_'
		}
		return r
	}, key)
}

// dueOf parses the fixed-width nanos prefix of a queue filename (its due time).
// Malformed names read as 0 (always due) so a legacy entry can never be stranded.
func dueOf(name string) int64 {
	i := strings.IndexByte(name, '-')
	if i <= 0 {
		return 0
	}
	n, err := strconv.ParseInt(name[:i], 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// A queue filename is `<due:019d>-<pid>.p<prio>[.k-<key>][.a<n>].txt`. Everything a
// drain needs to order, hold, and identify an entry is in the name — the queue is a
// directory of files claimed by rename, and a sidecar index would need its own crash
// story. Every field is optional on read: an entry written by an OLDER gtmux (no
// `.p`, no `.a`) parses as the default priority, zero attempts, and due now, so an
// in-flight upgrade drains its backlog instead of stranding it.

// prioOf parses the `.p<n>` field. The key is excluded from the search so a key can
// never be mistaken for the priority.
func prioOf(name string) int {
	if i := strings.Index(name, ".k-"); i >= 0 {
		name = name[:i]
	}
	i := strings.Index(name, ".p")
	if i < 0 || i+2 >= len(name) {
		return hqwake.PriorityDefault
	}
	n, err := strconv.Atoi(string(name[i+2]))
	if err != nil {
		return hqwake.PriorityDefault
	}
	return n
}

// attemptsOf parses the `.a<n>` field: how many times this entry has been pasted
// without its delivery being confirmed. A key can never be confused for it —
// sanitizeKey strips dots.
func attemptsOf(name string) int {
	base := identityOf(name)
	if base == strings.TrimSuffix(name, ".txt") {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSuffix(name, ".txt")[len(base)+2:])
	if err != nil {
		return 0
	}
	return n
}

// identityOf strips the attempt counter (and the extension), leaving the entry's
// STABLE identity. batchID hashes these, so a re-send of the same entries produces
// the same id no matter how many attempts it took.
func identityOf(name string) string {
	base := strings.TrimSuffix(name, ".txt")
	i := strings.LastIndex(base, ".a")
	if i < 0 {
		return base
	}
	if _, err := strconv.Atoi(base[i+2:]); err != nil {
		return base
	}
	return base[:i]
}

// withAttempt renames an entry to carry attempt n.
func withAttempt(name string, n int) string {
	return fmt.Sprintf("%s.a%d.txt", identityOf(name), n)
}

// batchID is the batch's short identity: 6 hex of the claimed entries' identities
// plus the payload. Those identities carry each entry's nanos, so a RE-SEND of an
// unconfirmed batch (which reclaims the same files) hashes identically, while a
// genuinely new batch with the same text does not. It is what the ack matches on —
// the line's own head is shared with every earlier wake of the same class about the
// same pane; the id is not.
func batchID(claims []string, payload string) string {
	h := sha256.New()
	for _, c := range claims {
		h.Write([]byte(identityOf(strings.TrimSuffix(filepath.Base(c), sendingSuffix))))
		h.Write([]byte{0})
	}
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))[:6]
}

// drainInto delivers the queue as ONE coalesced line. It MUST be called only after
// boxEmpty confirmed an empty box (the invariant). Each file is claimed by an atomic
// rename to `.sending` so a concurrent drainer can't re-deliver it; a lost claim just
// skips that file (at worst a coalesce splits across two lines — never a loss/dup).
// A claim is REMOVED only once the delivery is acked; otherwise it is returned to the
// queue for a later attempt.
func drainInto(x io, pane string) {
	reclaimOrphans()
	due := queuedNames(func(n string) bool { return dueOf(n) <= x.nowNano() })
	claimed, msgs, held := claimBatch(due)
	if len(msgs) == 0 {
		for _, c := range claimed {
			_ = os.Remove(c) // empty/unreadable — there is nothing to deliver or keep
		}
		return
	}

	payload := strings.Join(msgs, " · ")
	if held > 0 {
		payload += fmt.Sprintf(" · +%d more queued", held)
	}
	id := "#" + batchID(claimed, payload)
	payload += " · " + id

	switch deliverPayload(x, pane, payload, id) {
	case delivered:
		for _, c := range claimed {
			_ = os.Remove(c)
		}
		writeFailCount(0)
	case sendFailed:
		// Nothing reached the pane — return the batch untouched (no attempt spent,
		// no duplicate risk) and let a later drain retry it as often as it likes.
		for _, c := range claimed {
			_ = os.Rename(c, strings.TrimSuffix(c, sendingSuffix))
		}
		writeFailCount(readFailCount() + 1)
	case unacked:
		requeueUnacked(claimed)
		writeFailCount(readFailCount() + 1)
	}
}

// claimBatch claims up to one delivery's worth of due entries (highest priority
// first, oldest first within a priority) and returns the claims, their payloads, and
// how many due entries were held back for the next drain.
func claimBatch(due []string) (claimed, msgs []string, held int) {
	chars := 0
	for i, n := range due {
		if len(msgs) >= maxBatchLines || (len(msgs) > 0 && chars > maxBatchChars) {
			return claimed, msgs, len(due) - i
		}
		src := filepath.Join(queueDir(), n)
		dst := src + sendingSuffix
		if os.Rename(src, dst) != nil {
			continue // lost the claim to another drainer
		}
		claimed = append(claimed, dst)
		b, err := os.ReadFile(dst)
		if err != nil {
			continue
		}
		if s := strings.TrimSpace(string(b)); s != "" {
			msgs = append(msgs, s)
			chars += len(s)
		}
	}
	return claimed, msgs, 0
}

// requeueUnacked returns each claim for one more attempt, or drops it once it has
// been pasted maxAckAttempts times without a confirmation. See maxAckAttempts: by
// then the line has very probably been on HQ's screen three times over, and the
// wake-degraded escalation has already announced the doubt.
func requeueUnacked(claimed []string) {
	for _, c := range claimed {
		orig := strings.TrimSuffix(filepath.Base(c), sendingSuffix)
		if n := attemptsOf(orig) + 1; n < maxAckAttempts {
			_ = os.Rename(c, filepath.Join(queueDir(), withAttempt(orig, n)))
		} else {
			_ = os.Remove(c)
		}
	}
}

// outcome is what one delivery attempt achieved. The distinction between sendFailed
// (nothing reached the pane) and unacked (it may have) is what lets the retry policy
// be generous with the first and bounded with the second.
type outcome int

const (
	delivered  outcome = iota // confirmed on HQ's screen
	sendFailed                // tmux refused the paste or the submit — nothing landed
	unacked                   // pasted and submitted, but not confirmed
)

// deliverPayload types ONE batch and confirms it landed. Paste and submit are separate
// steps (`send-keys -l` of a long string mid-TUI errors "not in a mode" — the failure
// tmux.Paste exists to avoid), and the ack looks for the batch id in the pane's
// HISTORY: an id found in the DRAFT means the paste landed but the Enter did not,
// which is the opposite of delivered.
func deliverPayload(x io, pane, payload, id string) outcome {
	if err := x.paste(pane, payload); err != nil {
		return sendFailed
	}
	if err := x.enter(pane); err != nil {
		// One retry: a swallowed submit leaves our paste sitting in HQ's box, which
		// blocks every later drain at the draft guard.
		if err = x.enter(pane); err != nil {
			return sendFailed
		}
	}
	x.sleep()
	history, draft, _ := dispatch.SplitInputRegion(x.captureFull(pane))
	if dispatch.ContainsHead(history, id) && !dispatch.ContainsHead(draft, id) {
		return delivered
	}
	return unacked
}

func readFailCount() int {
	n, _ := strconv.Atoi(state.ReadMarker(failCountPath()))
	return n
}

func writeFailCount(n int) {
	if n <= 0 {
		state.Remove(failCountPath())
		return
	}
	_ = state.WriteMarker(failCountPath(), strconv.Itoa(n))
}
